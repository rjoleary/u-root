// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
)

func init() {
	buildGenerators["bb"] = bbGenerator{}
}

type bbGenerator struct {
}

func (g bbGenerator) generate() ([]file, error) {
	return nil, errors.New("not implemented yet")
}
