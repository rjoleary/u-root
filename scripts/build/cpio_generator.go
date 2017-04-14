// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
)

func init() {
	archiveGenerators["cpio"] = cpioGenerator{}
}

type cpioGenerator struct {
}

func (g cpioGenerator) generate(files []file) error {
	if *initialCpio != "" {
		f, err := ioutil.ReadFile(*initialCpio)
		if err != nil {
			log.Fatalf("%v", err)
		}

		cmd := exec.Command("sudo", "cpio", "-i", "-v")
		cmd.Dir = config.TempDir
		// Note: if you print Cmd out with %v after assigning cmd.Stdin, it will print
		// the whole cpio; so don't do that.
		if *debugFlag {
			cmd.Stdout = os.Stdout
		}
		debug("Run %v @ %v", cmd, cmd.Dir)

		// There's a bit of a tough problem here. There's lots of stuff owned by root in
		// these directories. They probably have to stay that way. But how do we create init
		// and do other things? For now, we're going to set the modes of select places to
		// 666 and remove a few things we know need to be removed.
		// It's hard to say what else to do.
		cmd.Stdin = bytes.NewBuffer(f)
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Printf("Unpacking %v: %v", *initialCpio, err)
		}
	}

	if !*existingInit {
		init := path.Join(config.TempDir, "init")
		// Must move config.TempDir/init to inito if one is not there.
		inito := path.Join(config.TempDir, "inito")
		if _, err := os.Stat(inito); err != nil {
			// WTF? did Ron forget about rename? Yuck!
			if err := syscall.Rename(init, inito); err != nil {
				log.Printf("%v", err)
			}
		} else {
			log.Printf("Not replacing %v because there is already one there.", inito)
		}

		// Build init
		cmd := exec.Command("go", "build", "-x", "-a", "-installsuffix", "cgo", "-ldflags", "'-s'", "-o", init, ".")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Dir = path.Join(config.Urootpath, "cmds/init")

		err := cmd.Run()
		if err != nil {
			log.Fatalf("%v\n", err)
		}
	}

	// These produce arrays of strings, the first element being the
	// directory to walk from.
	cpio := []string{
		goList,
		urootList,
	}

	for _, c := range cpio {
		if err := cpiop(c); err != nil {
			log.Printf("Things went south. TempDir is %v", config.TempDir)
			log.Fatalf("Bailing out near line 666")
		}
	}

	debug("Done all cpio operations")

	r, w, err := os.Pipe()
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	// First create the archive and put the device cpio in it.
	dev, err := ioutil.ReadFile(path.Join(config.Urootpath, devcpio))
	if err != nil {
		log.Fatalf("%v %v\n", dev, err)
	}

	debug("Creating initramf file")

	oname := fmt.Sprintf("/tmp/initramfs.%v_%v.cpio", config.Goos, config.Arch)
	if err := ioutil.WriteFile(oname, dev, 0600); err != nil {
		log.Fatalf("%v\n", err)
	}

	// Now use the append option for cpio to append to it.
	// That way we get one cpio.
	// We need sudo as there may be files created from an initramfs that
	// can only be read by root.
	cmd := exec.Command("sudo", "cpio", "-H", "newc", "-o", "-A", "-F", oname)
	cmd.Dir = config.TempDir
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	debug("Run %v @ %v", cmd, cmd.Dir)
	err = cmd.Start()
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	if err := lsr(config.TempDir, w); err != nil {
		log.Fatalf("%v\n", err)
	}
	w.Close()
	debug("Finished sending file list for initramfs cpio")
	err = cmd.Wait()
	if err != nil {
		log.Printf("%v\n", err)
	}
	debug("cpio for initramfs is done")
	defer func() {
		log.Printf("Output file is in %v\n", oname)
	}()
	return nil
}

func (g cpioGenerator) run() error {
	log.Fatal("not supported yet")
	return nil
}

// cpio copies a tree from one place to another, defined by a template.
func cpiop(c string) error {
	t := template.Must(template.New("filelist").Parse(c))
	var b bytes.Buffer
	if err := t.Execute(&b, config); err != nil {
		log.Fatalf("spec %v: %v\n", c, err)
	}

	n := strings.Split(b.String(), "\n")
	debug("cpiop: from %v, to %v, :%v:\n", n[0], n[1], n[2:])

	r, w, err := os.Pipe()
	if err != nil {
		log.Fatalf("%v\n", err)
	}
	cmd := exec.Command("sudo", "cpio", "--make-directories", "-p", path.Join(config.TempDir, n[1]))
	d := path.Clean(n[0])
	cmd.Dir = d
	cmd.Stdin = r
	cmd.Stdout = os.Stdout
	if *debugFlag {
		cmd.Stderr = os.Stderr
	}
	debug("Run %v @ %v", cmd, cmd.Dir)
	err = cmd.Start()
	if err != nil {
		log.Printf("%v\n", err)
	}

	for _, v := range n[2:] {
		debug("%v\n", v)
		err := filepath.Walk(path.Join(d, v), func(name string, fi os.FileInfo, err error) error {
			if err != nil {
				log.Printf(" WALK FAIL%v: %v\n", name, err)
				// That's ok, sometimes things are not there.
				return filepath.SkipDir
			}
			cn := strings.TrimPrefix(name, d+"/")
			if cn == ".git" {
				return filepath.SkipDir
			}
			fmt.Fprintf(w, "%v\n", cn)
			//log.Printf("c.dir %v %v %v\n", d, name, cn)
			return nil
		})
		if err != nil {
			log.Printf("%s: %v\n", v, err)
		}
	}
	w.Close()
	debug("Done sending files to external")
	err = cmd.Wait()
	if err != nil {
		log.Printf("%v\n", err)
	}
	debug("External cpio is done")
	return nil
}

func lsr(n string, w *os.File) error {
	n = n + "/"
	err := filepath.Walk(n, func(name string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		cn := strings.TrimPrefix(name, n)
		fmt.Fprintf(w, "%v\n", cn)
		return nil
	})
	return err
}
