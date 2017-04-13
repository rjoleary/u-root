package main

import (
	"fmt"
)

type listGenerator struct {
}

func (g listGenerator) generate(files <-chan file) {
	for f := range files {
		fmt.Printf("%v\t%d\t%q\n", f.mode, len(f.data), f.path)
	}
}

func (g listGenerator) run() error {
	fmt.Println("Nothing to run")
	return nil
}
