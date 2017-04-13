// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"
	"os/exec"
	"path"
)

type chrootGenerator struct {
}

func (g chrootGenerator) generate(files <-chan file) {
}

func (g chrootGenerator) run() error {
	// We need to populate the temp directory with dev.cpio. It's a chicken and egg thing;
	// we can't run init without, e.g., /dev/console and /dev/null.
	cmd := exec.Command("sudo", "cpio", "-i")
	cmd.Dir = config.TempDir
	// We have it in memory. Get a better way to do this!
	r, err := os.Open(path.Join(config.Urootpath, devcpio))
	if err != nil {
		return err
	}

	// OK, at this point, we know we can run as root. And, we're going to create things
	// we can only remove as root. So, we'll have to remove the directory with
	// extreme measures.
	cmd.Stdin, cmd.Stderr, cmd.Stdout = r, os.Stderr, os.Stdout
	debug("Run %v @ %v", cmd, cmd.Dir)
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Arrange to start init in the directory in a new namespace.
	// That should make all mounts go away when we're done.
	// On real kernels you can unshare without being root. Not on Linux.
	cmd = exec.Command("sudo", "unshare", "-m", "chroot", config.TempDir, "/init")
	cmd.Dir = config.TempDir
	cmd.Stdin, cmd.Stderr, cmd.Stdout = os.Stdin, os.Stderr, os.Stdout
	debug("Run %v @ %v", cmd, cmd.Dir)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Test failed, not removing %v: %v", config.TempDir, err)
		return err
	}

	return nil
}
