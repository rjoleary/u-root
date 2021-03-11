// Copyright 2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchdog

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

const daemonBin = "watchdogd"

// DaemonOpts contain options for the watchdog daemon.
type DaemonOpts struct {
	// Dev is the watchdog device. Ex: /dev/watchdog
	Dev string

	// When set to -1, uses the preset values.
	TimeoutSecs, PreTimeoutSecs int

	// KeepAliveSecs is the length of the keep alive interval.
	KeepAliveSecs int
}

// Run runs the watchdog on the current goroutine.
//
// It starts armed, and can be controlled with signals:
// - USR1: disarm
// - USR2: arm
//
// Cancelling the context will exit with the current armed/disarmed state.
func Run(ctx context.Context, opts *DaemonOpts) error {
	defer log.Println("Watchdog daemon quit")

	signals := make(chan os.Signal, 5)
	signal.Notify(signals, unix.SIGUSR1, unix.SIGUSR2)
	defer signal.Stop(signals)

	for {
		wd, err := Open(opts.Dev)
		if err != nil {
			return err
		}
		if opts.TimeoutSecs >= 0 {
			_, err := wd.SetTimeout(int32(opts.TimeoutSecs))
			if err != nil {
				wd.Close()
				return err
			}
		}
		if opts.PreTimeoutSecs >= 0 {
			_, err := wd.SetPreTimeout(int32(opts.PreTimeoutSecs))
			if err != nil {
				wd.Close()
				return err
			}
		}

		log.Println("Watchdog daemon armed")
	armed: // Loop while armed. SIGUSR1 to break.
		for {
			select {
			case <-time.After(time.Duration(opts.KeepAliveSecs) * time.Second):
				if err := wd.KeepAlive(); err != nil {
					log.Print("Failed to run keepalive watchdog")
				}
			case s := <-signals:
				if s == unix.SIGUSR1 {
					if err := wd.MagicClose(); err != nil {
						log.Printf("Failed to disarm watchdog: %v", err)
						// With this error, we don't
						// know if the watchdog is
						// armed or not. Keep petting
						// to be safe.
						continue
					}
					break armed
				}
			case <-ctx.Done():
				return wd.Close()
			}
		}
		wd.MagicClose()

		log.Println("Watchdog daemon disarmed")
	disarmed: // Loop while disarmed. SIGUSR2 to break.
		for {
			select {
			case s := <-signals:
				if s == unix.SIGUSR2 {
					break disarmed
				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// ForkDaemon runs the watchdog daemon as a child process. This assumes there
// is a binary called watchdogd.
func ForkDaemon(opts *DaemonOpts) error {
	cmd := exec.Command(daemonBin, "run",
		fmt.Sprintf("--dev=%s", opts.Dev),
		fmt.Sprintf("--timeout_secs=%d", opts.TimeoutSecs),
		fmt.Sprintf("--pre_timeout_secs=%d", opts.PreTimeoutSecs),
		fmt.Sprintf("--keep_alive_secs=%d", opts.KeepAliveSecs))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

// FindDaemonProcess returns the process id of the daemon.
func FindDaemonProcess() (*os.Process, error) {
	files, err := filepath.Glob("/proc/*/comm")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		// Ignore errors since /proc changes frequently.
		comm, _ := ioutil.ReadFile(f)
		// /proc files have a gratuitous newline.
		if string(comm) == daemonBin+"\n" {
			pidStr := filepath.Base(filepath.Dir(f))
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				return nil, fmt.Errorf("could not parse %q as a %q pid", pidStr, daemonBin)
			}
			return os.FindProcess(pid)
		}
	}
	return nil, fmt.Errorf("could not find %s", daemonBin)
}

func sendSignal(s os.Signal) error {
	p, err := FindDaemonProcess()
	if err != nil {
		return err
	}
	return p.Signal(s)
}

// DisarmDaemon sends a signal to the watchdog daemon to disarm.
func DisarmDaemon() error {
	return sendSignal(unix.SIGUSR1)
}

// ArmDaemon sends a signal to the watchdog deamon to arm.
func ArmDaemon() error {
	return sendSignal(unix.SIGUSR2)
}

// StopDaemon stops the daemon. It can be resumed with ContinueDaemon().
func StopDaemon() error {
	return sendSignal(unix.SIGSTOP)
}

// ContinueDaemon continues the daemon from a previous StopDaemon().
func ContinueDaemon() error {
	return sendSignal(unix.SIGCONT)
}
