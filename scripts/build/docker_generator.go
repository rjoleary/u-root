// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
)

func init() {
	archiveGenerators["docker"] = dockerGenerator{}
}

type dockerGenerator struct {
}

func (g dockerGenerator) generate(files <-chan file) {
	log.Fatal("not implemented yet")
}

func (g dockerGenerator) run() error {
	log.Fatal("not implemented yet")
	return nil
}
