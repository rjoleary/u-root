package qemu

import (
	"errors"
	"os"
	"regexp"
	"time"

	"github.com/rjoleary/goexpect"
)

var defaultTimeout = 5 * time.Second

type QEMU struct {
	// Path to the initramfs. Multiple initramfs can be used simultaneously.
	Init string

	InitRamfs string

	// Path to the bzimage kernel.
	Kernel string

	gExpect *expect.GExpect
}

func (q *QEMU) CmdLine() []string {
	args := []string{
		// TODO: these are specific to ryan's machine
		"qemu-system-x86_64",
		"-L", os.ExpandEnv("$HOME/repos/qemu/pc-bios"),
		"-m", "1024",
		"-M", "q35",
		"-enable-kvm",
		"-no-reboot",
		"-nographic",
		"-kernel", q.Kernel,
		"-append", "console=ttyS0 earlyprintk=ttyS0",
	}
	if q.InitRamfs != "" {
		args = append(args, "-initrd", q.InitRamfs)
	}
	return args
}

func (q *QEMU) Start() error {
	if q.gExpect != nil {
		return errors.New("QEMU already started")
	}
	var err error
	q.gExpect, _, err = expect.Spawn(q.CmdLine(), -1)
	return err
}

func (q *QEMU) Close() {
	q.gExpect.Close()
	q.gExpect = nil
}

func (q *QEMU) Send(in string) {
	q.gExpect.Send(in)
}

func (q *QEMU) Expect(search string) error {
	return q.ExpectTimeout(search, defaultTimeout)
}

func (q *QEMU) ExpectTimeout(search string, timeout time.Duration) error {
	return q.ExpectRETimeout(regexp.MustCompile(regexp.QuoteMeta(search)), timeout)
}

func (q *QEMU) ExpectRE(pattern *regexp.Regexp) error {
	return q.ExpectRETimeout(pattern, defaultTimeout)
}

func (q *QEMU) ExpectRETimeout(pattern *regexp.Regexp, timeout time.Duration) error {
	_, _, err := q.gExpect.Expect(pattern, timeout)
	return err
}
