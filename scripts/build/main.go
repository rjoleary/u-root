// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Synopsis:
//     ramfs [OPTIONS]
//
// Options:
//     -buildFormat:  one of src or bb (default "src")
//     -initialCpio:  an initial cpio image to build on
//     -d:            debug
//     -format:       one of chroot, cpio, docker or list (default "chroot")
//     -removedir:    remove the directory when done -- cleared if test fails
//     -run:          run the generated ramfs
//     -tmpdir:       tmpdir to use instead of ioutil.TempDir
//     -existingInit: if there is an existing init, do not replace it
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
)

var (
	buildFormat   = flag.String("buildFormat", "src", "one of src or bb")
	initialCpio   = flag.String("initialCpio", "", "An initial cpio image to build on")
	debugFlag     = flag.Bool("d", false, "debug")
	archiveFormat = flag.String("format", "chroot", "one of chroot, cpio, docker or list")
	removeDir     = flag.Bool("removedir", true, "remove the directory when done -- cleared if test fails")
	tempDir       = flag.String("tmpdir", "", "tmpdir to use instead of ioutil.TempDir")
	run           = flag.Bool("run", false, "run the generated ramfs")
	existingInit  = flag.Bool("existingInit", false, "if there is an existing init, do not replace it")
)

type copyfiles struct {
	dir  string
	spec string
}

const (
	devcpio   = "scripts/dev.cpio"
	urootPath = "src/github.com/u-root/u-root"
	urootCmds = "github.com/u-root/u-root/cmds"
)

var (
	// be VERY CAREFUL with these. If you have an empty line here it will
	// result in cpio copying the whole tree.
	goList = `{{.Goroot}}
go
pkg/include
VERSION.cache`
	urootList = `{{.Gopath}}
`
	config struct {
		Goroot    string
		Godotdot  string
		Godot     string
		Arch      string
		Goos      string
		Gopath    string
		Urootpath string
		TempDir   string
		Go        string
		Fail      bool
	}
	pkgList     []string
	dirs        = map[string]bool{}
	deps        = map[string]bool{}
	gorootFiles = map[string]bool{}
	urootFiles  = map[string]bool{}
	debug       = func(string, ...interface{}) {}
)

var (
	buildGenerators   = map[string]buildGenerator{}
	archiveGenerators = map[string]archiveGenerator{}
)

func globlist(s ...string) []string {
	// For each arg, use it as a Glob pattern and add any matches to the
	// package list. If there are no arguments, use [a-zA-Z]* as the glob pattern.
	var pat []string
	for _, v := range s {
		pat = append(pat, path.Join(config.Urootpath, "cmds", v))
	}
	if len(s) == 0 {
		pat = []string{path.Join(config.Urootpath, "cmds", "[a-zA-Z]*")}
	}
	return pat
}

// sad news. If I concat the Go cpio with the other cpios, for reasons I don't understand,
// the kernel can't unpack it. Don't know why, don't care. Need to create one giant cpio and unpack that.
// It's not size related: if the go archive is first or in the middle it still fails.
func main() {
	flag.Parse()
	if *debugFlag {
		debug = log.Printf
	}

	// Select the build generator.
	bGen, ok := buildGenerators[*buildFormat]
	if !ok {
		log.Fatal("invalid build generator")
	}

	// Select the archive generator.
	aGen, ok := archiveGenerators[*archiveFormat]
	if !ok {
		log.Fatal("invalid archive generator")
	}

	// Config
	os.Setenv("CGO_ENABLED", "0")
	guessgoarch()
	config.Go = ""
	config.Goos = "linux"
	guessgoroot()
	guessgopath()
	if config.Fail {
		log.Fatal("Setup failed")
	}

	pat := globlist(flag.Args()...)

	debug("Initial glob is %v", pat)
	for _, v := range pat {
		g, err := filepath.Glob(v)
		if err != nil {
			log.Fatalf("Glob error: %v", err)
		}
		// We have a set of absolute paths in g.  We can not
		// use absolute paths in go list, however, so we have
		// to adjust them.
		for i := range g {
			g[i] = path.Join(urootCmds, path.Base(g[i]))
		}
		pkgList = append(pkgList, g...)
	}

	debug("Initial pkgList is %v", pkgList)

	if config.TempDir == "" {
		var err error
		config.TempDir, err = ioutil.TempDir("", "u-root")
		if err != nil {
			log.Fatalf("%v", err)
		}
	}

	defer func() {
		// TODO:
		/*if removeDir {
			log.Printf("Removing %v\n", config.TempDir)
			// Wow, this one is *scary*
			cmd := exec.Command("sudo", "rm", "-rf", config.TempDir)
			cmd.Stderr, cmd.Stdout = os.Stderr, os.Stdout
			err = cmd.Run()
			if err != nil {
				log.Fatalf("%v", err)
			}
		}*/
	}()

	// Generate files
	files, err := bGen.generate()
	if err != nil {
		log.Fatal(err)
	}

	// Add device files
	files = append(files, []file{
		{"bin", []byte{}, 0755 | os.ModeDir, 0, 0, 0},
		{"dev", []byte{}, 0755 | os.ModeDir, 0, 0, 0},
		{"dev/console", []byte{}, 0644 | os.ModeDevice | os.ModeCharDevice, 0, 0, dev(5, 1)},
		{"dev/loop-control", []byte{}, 0600 | os.ModeDevice | os.ModeCharDevice, 0, 0, dev(10, 237)},
		{"dev/loop0", []byte{}, 0660 | os.ModeDevice, 0, 0, dev(7, 0)},
		{"dev/loop1", []byte{}, 0660 | os.ModeDevice, 0, 0, dev(7, 1)},
		{"dev/loop2", []byte{}, 0660 | os.ModeDevice, 0, 0, dev(7, 2)},
		{"dev/loop3", []byte{}, 0660 | os.ModeDevice, 0, 0, dev(7, 3)},
		{"dev/loop4", []byte{}, 0660 | os.ModeDevice, 0, 0, dev(7, 4)},
		{"dev/loop5", []byte{}, 0660 | os.ModeDevice, 0, 0, dev(7, 5)},
		{"dev/loop6", []byte{}, 0660 | os.ModeDevice, 0, 0, dev(7, 6)},
		{"dev/loop7", []byte{}, 0660 | os.ModeDevice, 0, 0, dev(7, 7)},
		{"dev/null", []byte{}, 0644 | os.ModeDevice | os.ModeCharDevice, 0, 0, dev(1, 3)},
		{"dev/ttyS0", []byte{}, 0644 | os.ModeDevice | os.ModeCharDevice, 0, 0, dev(7, 2)},
		{"etc", []byte{}, 0755 | os.ModeDir, 0, 0, 0},
		{"etc/localtime", []byte{}, 0644, 0, 0, 0},   // TODO: data
		{"etc/resolv.conf", []byte{}, 0644, 0, 0, 0}, // TODO: data
		{"lib64", []byte{}, 0755 | os.ModeDir, 0, 0, 0},
		{"tcz", []byte{}, 0755 | os.ModeDir, 0, 0, 0},
		{"tmp", []byte{}, 0755 | os.ModeDir, 0, 0, 0},
		{"usr", []byte{}, 0755 | os.ModeDir, 0, 0, 0},
		{"usr/lib", []byte{}, 0755 | os.ModeDir, 0, 0, 0},
	}...)

	// Sort files
	sort.Slice(files, func(i, j int) bool {
		if files[i].path == files[j].path {
			log.Println("warning: two files named", files[i].path)
		}
		return files[i].path < files[j].path
	})

	// Generate archive
	if err := aGen.generate(files); err != nil {
		log.Fatal(err)
	}

	// Optionally execute the archive
	if *run {
		if err := aGen.run(); err != nil {
			log.Fatal(err)
		}
	}
}
