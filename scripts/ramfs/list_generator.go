package main

import (
	"fmt"
)

type listGenerator struct {
}

func (g listGenerator) generate(fileChan <-chan file) {
	for f := range fileChan {
		if f.rdev == 0 {
			fmt.Printf("%v\t%d\t%q\n", f.mode, len(f.data), f.path)
		} else {
			fmt.Printf("%v\t%d, %d\t%q\n", f.mode, f.rdev>>8, f.rdev&0xff, f.path)
		}
	}
}

func (g listGenerator) run() error {
	fmt.Println("Nothing to run")
	return nil
}
