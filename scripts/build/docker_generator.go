// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
)

func init() {
	archiveGenerators["docker"] = dockerGenerator{}
}

type dockerGenerator struct {
}

func (g dockerGenerator) generate(files []file) error {
	return errors.New("not implemented yet")
}

func (g dockerGenerator) run() error {
	return errors.New("not implemented yet")
}
