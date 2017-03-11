// Copyright 2015-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// mkbb compiles Go packages into a busybox.
//
// Synopsis:
//	bb [ARGS...] [PACKAGES...]
//
// Description:
//      "$GOPATH/github.com/u-root/u-root/cmds/*"
//
// Options:
//     -leave_tmp: do not delete intermediate files
//     -v:         verbose mode
//     -vv:        extra verbose mode
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"text/template"

	"golang.org/x/tools/imports"
)

var (
	leaveTmp = flag.Boolean("leave_tmp", false, "do not delete intermediate files")
	verbose1 = flag.Boolean("v", false, "verbose mode")
	verbose2 = flag.Boolean("vv", false, "extra verbose mode")
)

var (
	v = log.New(ioutil.Discard, "", 0)
	vv = log.New(ioutil.Discard, "", 0)
)

const (
	initGo = `
package main
import (
	"log"
	"os"
	"path"
	"github.com/u-root/u-root/uroot"
)

func init() {
	// This one stat adds a bit of cost to each invocation (not much really)
	// but it allows us to merge init and sh. The 600K we save is worth it.
	if _, err := os.Stat("/proc/self"); err == nil {
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

func oneFile(dir, s string, fset *token.FileSet, f *ast.File) error {
	// Inspect the AST and change all instances of main()
	isMain := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.File:
			x.Name.Name = config.CmdName
		case *ast.FuncDecl:
			if x.Name.Name == "main" {
				x.Name.Name = "Main"
				isMain = true
			}

		case *ast.CallExpr:
			debug("%v %v\n", reflect.TypeOf(n), n)
			switch z := x.Fun.(type) {
			case *ast.SelectorExpr:
				// somebody tell me how to do this.
				sel := fmt.Sprintf("%v", z.X)
				// TODO: Need to have fixFlag and fixFlagVar
				// as the Var variation has name in the SECOND argument.
				if sel == "flag" {
					if ix, ok := fixFlag[z.Sel.Name]; ok {
						switch zz := x.Args[ix].(type) {
						case *ast.BasicLit:
							zz.Value = "\"" + config.CmdName + "." + zz.Value[1:]
						}
					}
				}
			}
		}
		return true
	})

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

type package_ string
type goFile string

func (p package_) convert() {
	err = filepath.Walk(p, func(name string, fi os.FileInfo, err error) {

	})
}

func (f goFile) convert() {

}

func start() {
	if *verbose {
		info = log.New(os.Stderr, "", log.LstdFlags)
	}

	// Create temp directory.
	tmpDir, err := ioutil.TempDir("", "Test")
	if err != nil {
		return err
	}
	defer func() {
		if !leave_tmp {
			info.Printf("left temp dir at %#v", tmpDir)
		} else {
			os.RemoveAll(tmpDir)
		}
	}()

	// Gather list of packages.
	packages := flag.Args()
	if len(packages) == 0 {
		matches, _ := filepath.Glob("github.com/u-root/u-root/cmds/*")
		if len(matches) == 0 {
			log.Fatal("no packages")
		}
		packages = matches
	}

	for _, p := packages() {
		package_(p).convert()
	}

}

func main() {
	flag.Parse()
	if err := start(); err != nil {
		log.Fatal(err)
	}
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
