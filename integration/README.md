# Integration Tests

This tests a core use cases for u-root: retrieving and kexec'ing a Linux kernel.

Run the test with:

    go test

## Requirements

- QEMU
  - Path and flags can be override with QEMU argument
  - Ex: `export TEST_QEMU="$USER/bin/qemu-system-x86_64" -L .`

## To Dos

1. Support testing on architectures other than x86

There are three parts:

1. Run the tests with "go run integration/run_tests.go". This will start the qemu instances.
2. Each test is its own init script written in Go. For example, one init script
   might test running the kexec command and another init script runs the
   shutdown command.
3. Use https://github.com/google/goexpect to verify the output of qemu's serial.
