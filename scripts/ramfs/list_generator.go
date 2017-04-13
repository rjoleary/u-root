package main

import (
	"fmt"
)

type listGenerator struct {
}

func (g listGenerator) generate(fileChan <-chan file) {
	count := 0
	totalSize := 0
	for f := range fileChan {
		count++
		if f.rdev == 0 {
			fmt.Printf("%v\t%d\t%q\n", f.mode, len(f.data), f.path)
			totalSize += len(f.data)
		} else {
			fmt.Printf("%v\t%d, %d\t%q\n", f.mode, f.rdev>>8, f.rdev&0xff, f.path)
		}
	}
	fmt.Println("Number of files:", count)
	fmt.Printf("Total size: %.1f MiB (%d bytes)", float64(totalSize)/1024/1024, totalSize)
}

func (g listGenerator) run() error {
	fmt.Println("Nothing to run")
	return nil
}
