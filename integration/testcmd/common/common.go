// Copyright 2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"archive/tar"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/u-root/u-root/pkg/mount"
	"github.com/u-root/u-root/pkg/tarutil"
	"golang.org/x/sys/unix"
)

const (
	envUse9P            = "UROOT_USE_9P"
	envNoKernelCoverage = "UROOT_NO_KERNEL_COVERAGE"

	sharedDir          = "/testdata"
	kernelCoverageFile = "/testdata/kernel_coverage.tar"
)

// gcovFilter filters on all files ending with a gcda or gcno extension.
func gcovFilter(hdr *tar.Header) bool {
	if hdr.Typeflag == tar.TypeDir {
		hdr.Mode = 0770
		return true
	}
	if (filepath.Ext(hdr.Name) == ".gcda" && hdr.Typeflag == tar.TypeReg) ||
		(filepath.Ext(hdr.Name) == ".gcno" && hdr.Typeflag == tar.TypeSymlink) {
		hdr.Mode = 0660
		return true
	}
	return false
}

// CollectKernelCoverage saves the kernel coverage report to a tar file.
func CollectKernelCoverage() {
	if err := collectKernelCoverage(kernelCoverageFile); err != nil {
		log.Println("Falied to collect kernel coverage: %v", err)
	}
}

func collectKernelCoverage(filename string) error {
	// Check if we are collecting kernel coverage.
	if os.Getenv(envNoKernelCoverage) == "1" {
		log.Print("Not collecting kernel coverage")
		return nil
	}
	gcovDir := "/sys/kernel/debug/gcov"
	if _, err := os.Stat(gcovDir); !os.IsNotExist(err) {
		log.Print("Not collecting kernel coverage because %q does not exit", gcovDir)
		return nil
	}
	if os.Getenv(envUse9P) != "1" {
		// 9p is required to rescue the file from the VM.
		return fmt.Errorf("Not collecting kernel coverage because filesystem is not 9p")
	}

	// Mount debugfs.
	if err := unix.Mount("debugfs", "/sys/kernel/debug", "debugfs", 0, ""); err != nil {
		return fmt.Errorf("Failed to mount debugfs: %v", err)
	}

	// Copy out the kernel code coverage.
	log.Print("Collecting kernel coverage...")
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	if err := tarutil.CreateTarFilter(f, []string{gcovDir}, []tarutil.Filter{gcovFilter}); err != nil {
		f.Close()
		return err
	}
	// Sync to "disk" because we are about to shut down the kernel.
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("Error syncing: %v", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("Error closing: %v", err)
	}
	return nil
}

// MountSharedDir mounts the directory shared with the VM test. A cleanup
// function is returned to unmount.
func MountSharedDir() (func(), error) {
	// Mount a disk and run the tests within.
	var (
		mp  *mount.MountPoint
		err error
	)
	if os.Getenv(envUse9P) == "1" {
		mp, err = mount.Mount("tmpdir", sharedDir, "9p", "", 0)
	} else {
		mp, err = mount.Mount("/dev/sda1", sharedDir, "vfat", "", unix.MS_RDONLY)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to mount test directory: %v", err)
	}
	return func() { mp.Unmount(0) }, nil
}
