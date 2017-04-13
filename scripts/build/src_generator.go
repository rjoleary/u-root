// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
)

func init() {
	buildGenerators["src"] = srcGenerator{}
}

type srcGenerator struct {
}

type srcDstPair struct {
	src, dst string
}

func (g srcGenerator) generate(fileChan chan<- file) {
	wg := sync.WaitGroup{}

	// Read all go source files of the selected packages along with all the
	// dependent source files.
	wg.Add(1)
	go func() {
		defer wg.Done()
		files, err := listGoFiles()
		if err != nil {
			log.Fatalf("%v", err)
		}
		for _, f := range files {
			data, err := ioutil.ReadFile(f.src)
			if err != nil {
				log.Fatalf("unable to read %q: %v", f.src, err)
			}
			fileChan <- file{
				path: f.dst,
				data: data,
				mode: os.FileMode(0444),
			}
		}
	}()

	// Compile the four binaries needed for the Go toolchain: go, compile,
	// link and asm.
	toolDir := fmt.Sprintf("go/pkg/tool/%v_%v", config.Goos, config.Arch)
	for _, v := range []srcDstPair{
		{"src/cmd/go", "go/bin/go"},
		{"src/cmd/compile", path.Join(toolDir, "compile")},
		{"src/cmd/link", path.Join(toolDir, "link")},
		{"src/cmd/asm", path.Join(toolDir, "asm")},
	} {
		wg.Add(1)
		go func(v srcDstPair) {
			defer wg.Done()
			srcPkgPath := path.Join(config.Goroot, v.src)
			outPath := path.Join(config.TempDir, v.dst)
			goBuild(srcPkgPath, outPath)
			data, err := ioutil.ReadFile(outPath)
			if err != nil {
				log.Fatalf("unable to read %q: %v", outPath, err)
			}
			fileChan <- file{
				path: v.dst,
				data: data,
				mode: os.FileMode(0555),
			}
		}(v)
	}

	wg.Wait()
}

// Build a Go binary.
func goBuild(srcPkgPath, outPath string) {
	buildArgs := []string{
		"build",
		// rebuild all from scratch
		"-a",
		// build in a separate directory with the "uroot" suffix
		"-installsuffix=uroot",
		// strip symbols (huge space savings)
		"-ldflags=-s -w",
		// output binary location
		"-o", outPath,
	}

	cmd := exec.Command("go", buildArgs...)
	cmd.Dir = srcPkgPath
	if o, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Building %v: %v, %v\n", srcPkgPath, string(o), err)
	}
}

// goListPkg takes one package name, and computes all the files it needs to
// build, separating them into Go tree files and uroot files. For now we just
// 'go list' but hopefully later we can do this programmatically.
func goListPkg(pkgName string) (*build.Package, error) {
	// Perform a breadth-first-search to find all the dependencies.
	p, err := build.Import(pkgName, path.Join(config.Gopath, "src/github.com/u-root/u-root"), 0)
	if err != nil {
		return nil, err
	}

	debug("%v, %v %v %v", p, p.GoFiles, p.SFiles, p.HFiles)
	for _, v := range append(append(p.GoFiles, p.SFiles...), p.HFiles...) {
		if p.Goroot {
			gorootFiles[path.Join(p.ImportPath, v)] = true
		} else {
			urootFiles[path.Join(p.ImportPath, v)] = true
		}
	}

	return p, nil
}

// listGoFiles determines the list of Go source files for inclusion.
func listGoFiles() ([]srcDstPair, error) {
	for _, pkgName := range pkgList {
		p, err := goListPkg(pkgName)
		if err != nil {
			log.Printf("ignoring %q due to dependency error: %v", pkgName, err)
			continue
		}
		debug("cmd p is %v", p)
		for _, v := range p.Imports {
			deps[v] = true
		}
	}

	for v := range deps {
		if _, err := goListPkg(v); err != nil {
			log.Fatalf("%v", err)
		}
	}

	files := []srcDstPair{}
	for v := range gorootFiles {
		goList += "\n" + path.Join("src", v)
		files = append(files, srcDstPair{path.Join(config.Goroot, "src", v), path.Join("src", v)})
	}
	for v := range urootFiles {
		urootList += "\n" + path.Join("src", v)
		files = append(files, srcDstPair{path.Join(config.Gopath, "src", v), path.Join("src", v)})
	}
	return files, nil
}
