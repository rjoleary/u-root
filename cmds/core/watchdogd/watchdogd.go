// Copyright 2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// watchdogd is a background daemon for petting the watchdog.
//
// Synopsis:
//     watchdogd run
//         Run the watchdogd in a child process (does not daemonize).
//     watchdogd pid
//         Print the pid of the running watchdogd
//     watchdogd arm
//         Send a signal to arm the running watchdog.
//     watchdogd disarm
//         Send a signal to disarm the running watchdog.
//
// Options:
//     --dev DEV: Device (default /dev/watchdog)
//     --timeout_secs: Seconds before timing out (default -1)
//     --pre_timeout_secs: Seconds for pretimeout (default -1)
//     --keep_alive_secs: Seconds between issuing keepalive (default 10)
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	flag "github.com/spf13/pflag"
	"github.com/u-root/u-root/pkg/watchdog"
)

var (
	dev            = flag.String("dev", "/dev/watchdog", "device")
	timeoutSecs    = flag.Int("timeout_secs", -1, "seconds before timing out")
	preTimeoutSecs = flag.Int("pre_timeout_secs", -1, "seconds for pretimeout")
	keepAliveSecs  = flag.Int("keep_alive_secs", -1, "seconds between issuing keepalive")
)

func usage() {
	flag.Usage()
	fmt.Print(`watchdogd run
    Run the watchdogd daemon in a child process (does not daemonize).
watchdogd pid
    Print the pid of the running watchdogd
watchdogd arm
    Send a signal to arm the running watchdog.
watchdogd disarm
    Send a signal to disarm the running watchdog.
`)
	os.Exit(1)
}

func runCommand() error {
	flag.Parse()
	if flag.NArg() < 1 {
		usage()
	}

	switch flag.Arg(0) {
	case "run":
		return watchdog.Run(context.Background(), &watchdog.DaemonOpts{
			Dev:            *dev,
			TimeoutSecs:    *timeoutSecs,
			PreTimeoutSecs: *preTimeoutSecs,
			KeepAliveSecs:  *keepAliveSecs,
		})
	case "pid":
		p, err := watchdog.FindDaemonProcess()
		if err != nil {
			return err
		}
		fmt.Println(p.Pid)
	case "arm":
		return watchdog.ArmDaemon()
	case "disarm":
		return watchdog.DisarmDaemon()
	}
	return nil
}

func main() {
	if err := runCommand(); err != nil {
		log.Fatal(err)
	}
}
