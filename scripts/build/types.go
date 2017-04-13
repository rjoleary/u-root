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
	generate(files chan<- file)
}

// Append files to the archive.
type archiveGenerator interface {
	generate(files <-chan file)
	run() error
}
