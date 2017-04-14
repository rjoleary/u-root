// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
)

type file struct {
	path string
	data []byte
	mode os.FileMode
	uid  uint32
	gid  uint32
	rdev uint64
}

// Generates files for inclusion into the archive.
type buildGenerator interface {
	generate() ([]file, error)
}

// Create an archive given a slice of files.
type archiveGenerator interface {
	generate([]file) error
	run() error
}
