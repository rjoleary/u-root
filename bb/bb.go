// Copyright 2015-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// bb converts standalone u-root tools to shell builtins.
// It copies and converts a set of u-root utilities into a directory called bbsh.
// It assumes nothing; all files it needs are always copied, no matter what
// is in bbsh.
// bb needs to know where the uroot you are using is so it can find command source.
// UROOT=/home/rminnich/projects/u-root/u-root/
// bb needs to know the arch:
// GOARCH=amd64
// bb needs to know where the tools are, and they are in two places, the place it created them
// and the place where packages live:
// GOPATH=/home/rminnich/projects/u-root/u-root/bb/bbsh:/home/rminnich/projects/u-root/u-root
// bb needs to have a GOROOT
// GOROOT=/home/rminnich/projects/u-root/go1.5/go/
// There are no defaults.
package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"golang.org/x/tools/imports"
)

const (
	cmdFunc = `package main
import _ "github.com/u-root/u-root/bb/bbsh/cmds/{{.CmdName}}"
`
	initGo = `
package main
import (
	"log"
	"os"
	"path"
	"github.com/u-root/u-root/uroot"
	"github.com/u-root/u-root/bb/bbshare"
)

func runInit() bool {
	// This getpid adds a bit of cost to each invocation (not much really)
	// but it allows us to merge init and sh. The 600K we save is worth it.
	if os.Args[0] != "/init" {
		return false
	}
	if os.Getpid() != 1 {
		log.Printf("Skipping root file system setup since /init is not pid 1")
		return false
	}
	install()
	uroot.Rootfs()
	return true
}

func install() {
	for _, v := range bbshare.PkgNames() {
		t := path.Join("/bin", v)
		if err := os.Symlink("/init", t); err != nil {
			log.Printf("Symlink init to %v: %v", t, err)
		}
	}
}

func main() {
	if runInit() {
		return
	}

	if err := bbshare.Run(path.Base(os.Args[0])); err != nil {
		if len(os.Args) == 2 && os.Args[1] == "install" {
			install()
			return
		}
		log.Println(err)
		log.Println("Valid package names are:", bbshare.PkgNames())
	}
}
`
)

func debugPrint(f string, s ...interface{}) {
	log.Printf(f, s...)
}

func nodebugPrint(f string, s ...interface{}) {
}

const cmds = "cmds"

var (
	debug      = nodebugPrint
	defaultCmd = []string{
		"cat",
		"cmp",
		"comm",
		"cp",
		"date",
		"dd",
		"dmesg",
		"echo",
		"freq",
		"grep",
		"ip",
		//"kexec",
		"ls",
		"mkdir",
		"mount",
		"netcat",
		"ping",
		"printenv",
		"rm",
		"seq",
		"srvfiles",
		"tcz",
		"uname",
		"uniq",
		"unshare",
		"wc",
		"wget",
	}

	// fixFlag tells by existence if an argument needs to be fixed.
	// The value tells which argument.
	fixFlag = map[string]int{
		"Bool":        0,
		"BoolVar":     1,
		"Duration":    0,
		"DurationVar": 1,
		"Float64":     0,
		"Float64Var":  1,
		"Int":         0,
		"Int64":       0,
		"Int64Var":    1,
		"IntVar":      1,
		"String":      0,
		"StringVar":   1,
		"Uint":        0,
		"Uint64":      0,
		"Uint64Var":   1,
		"UintVar":     1,
		"Var":         1,
	}
	dumpAST = flag.Bool("D", false, "Dump the AST")
)

var config struct {
	Args     []string
	CmdName  string
	FullPath string
	Src      string
	Uroot    string
	Cwd      string
	Bbsh     string

	Goroot    string
	Gosrcroot string
	Arch      string
	Goos      string
	Gopath    string
	TempDir   string
	Go        string
	Debug     bool
	Fail      bool
}

func parseType(src string) (ast.Expr, error) {
	e, err := parser.ParseExpr("new(" + src + ")")
	if err != nil {
		return nil, err
	}
	return e.(*ast.CallExpr).Args[0], nil
}

func oneFile(dir, s string, fset *token.FileSet, f *ast.File, fs []*ast.File) error {
	// Perform type inference on the file.
	// See: https://github.com/golang/example/tree/master/gotypes#identifier-resolution
	log.Println("config.CmdName", config.CmdName)
	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check(dir, fset, []*ast.File{f}, nil)
	if err != nil {
		// TODO
		//log.Fatal(err) // type error
		log.Print(err)
		//return nil
	}
	log.Print(pkg.Scope())

	// Change the package name.
	//
	// Before:
	//     package ...
	//
	// After:
	//     package ${config.CmdName}
	f.Name.Name = config.CmdName

	// Add the bbshare import.
	//
	// Before:
	//      package ...
	//      ...
	//
	// After:
	//      package ...
	//      import "github.com/u-root/u-root/bb/bbshare"
	//      ...
	importBBShare := &ast.GenDecl{
		TokPos: f.Package,
		Tok: token.IMPORT,
		Specs: []ast.Spec {
			&ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind: token.STRING,
					Value: "\"github.com/u-root/u-root/bb/bbshare\"",
				},
			},
		},
	}
	f.Decls = append([]ast.Decl{importBBShare}, f.Decls...)

	// Translate the init functions.
	//
	// Before:
	//     func init() {
	//         ...
	//     }
	//
	// After:
	//     func init() {
	//         bbshare.AddInit("${config.CmdName}", func() {
	//             ...
	//         })
	//     }
	ast.Inspect(f, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok && funcDecl.Name.Name == "init" {
			// Replace the body.
			body := funcDecl.Body;
			funcDecl.Body = &ast.BlockStmt {
				List: []ast.Stmt{
					&ast.ExprStmt {
						X: &ast.CallExpr {
							Fun: &ast.SelectorExpr {
								X: ast.NewIdent("bbshare"),
								Sel: ast.NewIdent("AddInit"),
							},
							Args: []ast.Expr {
								&ast.BasicLit {
									Kind: token.STRING,
									Value: "\"" + config.CmdName + "\"",
								},
								&ast.FuncLit {
									Type: &ast.FuncType {
										Params: &ast.FieldList {},
									},
									Body: body,
								},
							},
						},
					},
				},
			}


		}
		return true
	})

	// Translate variable initializations.
	//
	// Before:
	//     var (
	//         x0 = e0
	//         x1 = e1
	//         ...
	//         xn = en
	//     )
	//
	// After:
	//     var (
	//         x0 t0
	//         x1 t1
	//         ...
	//         xn tn
	//     )
	//     func init() {
	//         bbshare.AddVarInit("${config.CmdName}", func() {
	//             x0 = e0
	//             x1 = e1
	//             ...
	//             xn = en
	//         })
	//     }
	valueSpecs := []*ast.ValueSpec{}
	for _, decl := range f.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
			for _, spec := range genDecl.Specs {
				if spec, ok := spec.(*ast.ValueSpec); ok {
					valueSpecs = append(valueSpecs, spec)
				}
			}
		}
	}
	assignStmts := []ast.Stmt{}
	for _, valueSpec := range valueSpecs {
		if valueSpec.Values == nil {
			continue // declaration, but no initializer
		}
		assignStmt := &ast.AssignStmt{
			Tok: token.ASSIGN,
			Rhs: valueSpec.Values,
		}
		for _, name := range valueSpec.Names {
			assignStmt.Lhs = append(assignStmt.Lhs, name)
		}
		assignStmts = append(assignStmts, assignStmt)
	}
	// TODO: does not work for weird tuples and iota
	for _, valueSpec := range valueSpecs {
		if valueSpec.Values == nil {
			continue // declaration, but no initializer
		}
		if valueSpec.Names[0].Name == "_" {
			valueSpec.Values = []ast.Expr{
				&ast.BasicLit {
					Kind: token.STRING,
					Value: "\"IGNORE\"",
				},
			}
			continue
		}
		t := pkg.Scope().Lookup(valueSpec.Names[0].Name)
		if t == nil || t.Type() == nil {
			log.Print("Warning: cannot resolve a type")
			continue
		}
		e, err := parseType(t.Type().String())
		if err != nil {
			return err
		}
		valueSpec.Values = nil
		valueSpec.Type = e
	}
	f.Decls = append(f.Decls, &ast.FuncDecl{
		Name: ast.NewIdent("init"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{},
			Results: nil,
		},
		Body: &ast.BlockStmt {
			List: []ast.Stmt{
				&ast.ExprStmt {
					X: &ast.CallExpr {
						Fun: &ast.SelectorExpr {
							X: ast.NewIdent("bbshare"),
							Sel: ast.NewIdent("AddVarInit"),
						},
						Args: []ast.Expr {
							&ast.BasicLit {
								Kind: token.STRING,
								Value: "\"" + config.CmdName + "\"",
							},
							&ast.FuncLit {
								Type: &ast.FuncType {
									Params: &ast.FieldList {},
								},
								Body: &ast.BlockStmt {
									List: assignStmts,
								},
							},
						},
					},
				},
			},
		},
	})

	// Translate the main function.
	//
	// Before:
	//     func main() {
	//         ...
	//     }
	//
	// After:
	//     func init() {
	//         bbshare.AddMain("${config.CmdName}", func() {
	//             ...
	//         })
	//     }
	isMain := false
	ast.Inspect(f, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok && funcDecl.Name.Name == "main" {
			isMain = true

			// Replace the name.
			funcDecl.Name.Name = "init"

			// Replace the body.
			body := funcDecl.Body;
			funcDecl.Body = &ast.BlockStmt {
				List: []ast.Stmt{
					&ast.ExprStmt {
						X: &ast.CallExpr {
							Fun: &ast.SelectorExpr {
								X: ast.NewIdent("bbshare"),
								Sel: ast.NewIdent("AddMain"),
							},
							Args: []ast.Expr {
								&ast.BasicLit {
									Kind: token.STRING,
									Value: "\"" + config.CmdName + "\"",
								},
								&ast.FuncLit {
									Type: &ast.FuncType {
										Params: &ast.FieldList {},
									},
									Body: body,
								},
							},
						},
					},
				},
			}
		}
		return true
	})

	if *dumpAST {
		ast.Fprint(os.Stderr, fset, f, nil)
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		panic(err)
	}
	debug("%s", buf.Bytes())
	out := string(buf.Bytes())

	// fix up any imports. We may have forced the issue
	// with os.Args
	opts := imports.Options{
		Fragment:  true,
		AllErrors: true,
		Comments:  true,
		TabIndent: true,
		TabWidth:  8,
	}
	fullCode, err := imports.Process("commandline", []byte(out), &opts)
	if err != nil {
		log.Fatalf("bad parse: '%v': %v", out, err)
	}

	of := path.Join(dir, path.Base(s))
	if err := ioutil.WriteFile(of, []byte(fullCode), 0666); err != nil {
		log.Fatalf("%v\n", err)
	}

	// fun: must write the file first so the import fixup works :-)
	if isMain {
		// Write the file to interface to the command package.
		t := template.Must(template.New("cmdFunc").Parse(cmdFunc))
		var b bytes.Buffer
		if err := t.Execute(&b, config); err != nil {
			log.Fatalf("spec %v: %v\n", cmdFunc, err)
		}
		fullCode, err := imports.Process("commandline", []byte(b.Bytes()), &opts)
		if err != nil {
			log.Fatalf("bad parse: '%v': %v", out, err)
		}
		if err := ioutil.WriteFile(path.Join(config.Bbsh, "cmd_"+config.CmdName+".go"), fullCode, 0444); err != nil {
			log.Fatalf("%v\n", err)
		}
	}

	return nil
}

func oneCmd() {
	// Create the directory for the package.
	// For now, ./cmds/<package name>
	packageDir := path.Join(config.Bbsh, "cmds", config.CmdName)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		log.Fatalf("Can't create target directory: %v", err)
	}

	fset := token.NewFileSet()
	config.FullPath = path.Join(config.Uroot, cmds, config.CmdName)
	p, err := parser.ParseDir(fset, config.FullPath, nil, 0)
	if err != nil {
		panic(err)
	}

	for _, f := range p {
		fs := []*ast.File{}
		for _, v := range f.Files {
			fs = append(fs, v)
		}
		for n, v := range f.Files {
			oneFile(packageDir, n, fset, v, fs)
		}
	}
}

func main() {
	doConfig()

	if err := os.MkdirAll(config.Bbsh, 0755); err != nil {
		log.Fatalf("%v", err)
	}

	if len(flag.Args()) > 0 {
		config.Args = []string{}
		for _, v := range flag.Args() {
			v = path.Join(config.Uroot, "cmds", v)
			g, err := filepath.Glob(v)
			if err != nil {
				log.Fatalf("Glob error: %v", err)
			}

			for i := range g {
				g[i] = path.Base(g[i])
			}
			config.Args = append(config.Args, g...)
		}
	}

	for _, v := range config.Args {
		// Yes, gross. Fix me.
		config.CmdName = v
		oneCmd()
	}

	if err := ioutil.WriteFile(path.Join(config.Bbsh, "init.go"), []byte(initGo), 0644); err != nil {
		log.Fatalf("%v\n", err)
	}
	// copy all shell files

	/*err = filepath.Walk(path.Join(config.Uroot, cmds, "rush"), func(name string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		b, err := ioutil.ReadFile(name)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(path.Join(config.Bbsh, fi.Name()), b, 0644); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Fatalf("%v", err)
	}*/

	buildinit()
	ramfs()
}
