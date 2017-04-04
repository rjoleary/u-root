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
	"go/parser"
	"go/token"
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
import "github.com/u-root/u-root/bb/bbsh/cmds/{{.CmdName}}"
func _forkbuiltin_{{.CmdName}}(c *Command) (err error) {
{{.CmdName}}.Main()
return
}

func init() {
	addForkBuiltIn("{{.CmdName}}", _forkbuiltin_{{.CmdName}})
}
`
	initGo = `
package main
import (
	"log"
	"os"
	"path"
	"github.com/u-root/u-root/uroot"
)

func init() {
	// This getpid adds a bit of cost to each invocation (not much really)
	// but it allows us to merge init and sh. The 600K we save is worth it.
	if os.Args[0] != "/init" {
		log.Printf("Skipping root file system setup since we are not /init")
		return
	}
	if os.Getpid() != 1 {
		log.Printf("Skipping root file system setup since /init is not pid 1")
		return
	}
	uroot.Rootfs()

	for n := range forkBuiltins {
		t := path.Join("/ubin", n)
		if err := os.Symlink("/init", t); err != nil {
			log.Printf("Symlink /init to %v: %v", t, err)
		}
	}
	return
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

func oneFile(dir, s string, fset *token.FileSet, f *ast.File) error {
	// Change the package name
	f.Name.Name = config.CmdName

	// Add bbshare import.
	//AddImport(fset, f, "github.com/u-root/u-root/bb/bbshare"
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
	//f.Imports = append(f.Imports, &ast.ImportSpec{nil, nil, &bbshare, nil, 0})
	//importBBShare := &ast.GenDecl{
		//TokPos
	/*
     8  .  Decls: []ast.Decl (len = 13) {
     9  .  .  0: *ast.GenDecl {
    10  .  .  .  Doc: nil
    11  .  .  .  TokPos: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/fileinfo_linux.go:7:1
    12  .  .  .  Tok: import
    13  .  .  .  Lparen: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/fileinfo_linux.go:7:8
    14  .  .  .  Specs: []ast.Spec (len = 7) {
    15  .  .  .  .  0: *ast.ImportSpec {
    16  .  .  .  .  .  Doc: nil
    17  .  .  .  .  .  Name: nil
    18  .  .  .  .  .  Path: *ast.BasicLit {
    19  .  .  .  .  .  .  ValuePos: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/fileinfo_linux.go:8:2
    20  .  .  .  .  .  .  Kind: STRING
    21  .  .  .  .  .  .  Value: "\"fmt\""
    22  .  .  .  .  .  }
    23  .  .  .  .  .  Comment: nil
    24  .  .  .  .  .  EndPos: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/fileinfo_linux.go:8:7
    25  .  .  .  .  }
    */

	//ast.SortImports(fset, f)

	// Inspect the AST and change all instances of main()
	isMain := false

	// Before:
	//     func main() {
	//         ...
	//     }
	//
	// After:
	//     func init() {
	//         bbshare.addMain("$PACKAGE", func() {
	//             ...
	//         })
	//     }
	ast.Inspect(f, func(n ast.Node) bool {
		if x, ok := n.(*ast.FuncDecl); ok {
			if x.Name.Name == "main" {
				x.Name.Name = "init"
				isMain = true
			}

			*ast.FuncDecl {
Doc: nil
Recv: nil
Name: *ast.Ident {
.  NamePos: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:76:6
.  Name: "init"
.  Obj: nil
}
Type: *ast.FuncType {
.  Func: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:76:1
  1110  .  .  .  .  Params: *ast.FieldList {
  1111  .  .  .  .  .  Opening: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:76:10
  1112  .  .  .  .  .  List: nil
  1113  .  .  .  .  .  Closing: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:76:11
  1114  .  .  .  .  }
  1115  .  .  .  .  Results: nil
  1116  .  .  .  }
  1117  .  .  .  Body: *ast.BlockStmt {
  1118  .  .  .  .  Lbrace: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:76:13
  1119  .  .  .  .  List: []ast.Stmt (len = 1) {
  1120  .  .  .  .  .  0: *ast.ExprStmt {
  1121  .  .  .  .  .  .  X: *ast.CallExpr {
  1122  .  .  .  .  .  .  .  Fun: *ast.SelectorExpr {
  1123  .  .  .  .  .  .  .  .  X: *ast.Ident {
  1124  .  .  .  .  .  .  .  .  .  NamePos: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:77:2
  1125  .  .  .  .  .  .  .  .  .  Name: "bbshare"
  1126  .  .  .  .  .  .  .  .  .  Obj: nil
  1127  .  .  .  .  .  .  .  .  }
  1128  .  .  .  .  .  .  .  .  Sel: *ast.Ident {
  1129  .  .  .  .  .  .  .  .  .  NamePos: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:77:10
  1130  .  .  .  .  .  .  .  .  .  Name: "addMain"
  1131  .  .  .  .  .  .  .  .  .  Obj: nil
  1132  .  .  .  .  .  .  .  .  }
  1133  .  .  .  .  .  .  .  }
  1134  .  .  .  .  .  .  .  Lparen: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:77:17
  1135  .  .  .  .  .  .  .  Args: []ast.Expr (len = 2) {
  1136  .  .  .  .  .  .  .  .  0: *ast.BasicLit {
  1137  .  .  .  .  .  .  .  .  .  ValuePos: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:77:18
  1138  .  .  .  .  .  .  .  .  .  Kind: STRING
  1139  .  .  .  .  .  .  .  .  .  Value: "\"$PACKAGE\""
  1140  .  .  .  .  .  .  .  .  }
  1141  .  .  .  .  .  .  .  .  1: *ast.FuncLit {
  1142  .  .  .  .  .  .  .  .  .  Type: *ast.FuncType {
  1143  .  .  .  .  .  .  .  .  .  .  Func: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:77:30
  1144  .  .  .  .  .  .  .  .  .  .  Params: *ast.FieldList {
  1145  .  .  .  .  .  .  .  .  .  .  .  Opening: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:77:34
  1146  .  .  .  .  .  .  .  .  .  .  .  List: nil
  1147  .  .  .  .  .  .  .  .  .  .  .  Closing: /usr/local/google/home/ryanoleary/go/src/github.com/u-root/u-root/cmds/ls/ls.go:77:35
  1148  .  .  .  .  .  .  .  .  .  .  }
  1149  .  .  .  .  .  .  .  .  .  .  Results: nil
  1150  .  .  .  .  .  .  .  .  .  }

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
		for n, v := range f.Files {
			oneFile(packageDir, n, fset, v)
		}
	}
}

func main() {
	var err error
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

	err = filepath.Walk(path.Join(config.Uroot, cmds, "rush"), func(name string, fi os.FileInfo, err error) error {
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
	}

	buildinit()
	ramfs()
}
