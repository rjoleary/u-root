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

// dev returns the device number given the major and minor numbers.
func dev(major, minor uint64) uint64 {
	return major<<8 + minor
}

// major returns the device number's major number.
func major(dev uint64) uint64 {
	return dev >> 8
}

// minor returns the device number's minor number.
func minor(dev uint64) uint64 {
	return dev & 0xff
}
