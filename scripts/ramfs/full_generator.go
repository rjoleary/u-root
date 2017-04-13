package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
)

type fullGenerator struct {
}

func (g fullGenerator) generate(files chan<- file) {
	wg := sync.WaitGroup{}

	f, err := listGoFiles()
	if err != nil {
		log.Fatalf("%v", err)
	}
	for _, v := range f {
		files <- file{
			path: v,
			mode: os.FileMode(0444),
		}
	}

	// Compile the four binaries needed for the Go toolchain: go, compile,
	// link and asm.
	toolDir := path.Join(config.TempDir, fmt.Sprintf("go/pkg/tool/%v_%v", config.Goos, config.Arch))
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
			files <- file{
				path: v.dst,
				data: data,
				mode: os.FileMode(0555),
			}
		}(v)
	}

	wg.Wait()
	close(files)
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
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0") // TODO: set CGO_ENABLED in main
	if o, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Building %v: %v, %v\n", srcPkgPath, string(o), err)
	}
}

type srcDstPair struct {
	src, dst string
}

// goListPkg takes one package name, and computes all the files it needs to
// build, separating them into Go tree files and uroot files. For now we just
// 'go list' but hopefully later we can do this programmatically.
func goListPkg(pkgName string) (*goDirs, error) {
	cmd := exec.Command("go", "list", "-json", pkgName)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0") // TODO: set CGO_ENABLED in one place
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var p goDirs
	if err := json.Unmarshal(out, &p); err != nil {
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

	return &p, nil
}

// listGoFiles determines the list of Go source files for inclusion.
func listGoFiles() ([]string, error) {
	// For each directory in pkgList, add its files and all its
	// dependencies. It would be nice to run go list -json with lots of
	// package names but it produces invalid JSON. It produces a stream
	// that is {}{}{} at the top level and the decoders don't like that.
	// TODO: this is possible with json.Decoder
	deps := map[string]bool{}
	for _, pkgName := range pkgList {
		p, err := goListPkg(pkgName)
		if err != nil {
			log.Printf("ignoring %q due to go list error: %v", pkgName, err)
			continue
		}
		debug("cmd p is %v", p)
		for _, v := range p.Deps {
			deps[v] = true
		}
	}

	for v := range deps {
		if _, err := goListPkg(v); err != nil {
			log.Fatalf("%v", err)
		}
	}

	files := []string{}
	for v := range gorootFiles {
		goList += "\n" + path.Join("src", v)
		files = append(files, v)
	}
	for v := range urootFiles {
		urootList += "\n" + path.Join("src", v)
		files = append(files, v)
	}
	return files, nil
}
