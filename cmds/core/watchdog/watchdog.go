// Copyright 2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// watchdog interacts with /dev/watchdog.
//
// Synopsis:
//     watchdog keepalive
//         Pet the watchdog. This resets the time left back to the timeout.
//     watchdog set[pre]timeout SECONDS
//         Set the watchdog timeout or pretimeout
//     watchdog get[pre]timeout
//         Print the watchdog timeout or pretimeout
//     watchdog gettimeleft
//         Print the amount of time left.
//
// Options:
//     --dev DEV: Device (default /dev/watchdog)
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	flag "github.com/spf13/pflag"
	"github.com/u-root/u-root/pkg/watchdog"
)

var (
	dev = flag.String("dev", "/dev/watchdog", "device")
)

func usage() {
	flag.Usage()
	fmt.Print(`watchdog keepalive
    Pet the watchdog. This resets the time left back to the timeout.
watchdog set[pre]timeout SECONDS
    Set the watchdog timeout or pretimeout.
watchdog get[pre]timeout
    Print the watchdog timeout or pretimeout.
watchdog gettimeleft
    Print the amount of time left.
`)
	os.Exit(1)
}

func runCommand() error {
	flag.Parse()
	if flag.NArg() < 1 {
		usage()
	}

	wd, err := watchdog.Open(*dev)
	if err != nil {
		return err
	}
	defer func() {
		if err := wd.Close(); err != nil {
			log.Printf("Failed to close watchdog: %v", err)
		}
	}()

	switch flag.Arg(0) {
	case "keepalive":
		if err := wd.KeepAlive(); err != nil {
			return err
		}
	case "settimeout":
		i, err := strconv.Atoi(flag.Arg(1))
		if err != nil {
			return err
		}
		if _, err := wd.SetTimeout(int32(i)); err != nil {
			return err
		}
	case "setpretimeout":
		i, err := strconv.Atoi(flag.Arg(1))
		if err != nil {
			return err
		}
		if _, err := wd.SetPreTimeout(int32(i)); err != nil {
			return err
		}
	case "gettimeout":
		i, err := wd.Timeout()
		if err != nil {
			return err
		}
		fmt.Println(i)
	case "getpretimeout":
		i, err := wd.PreTimeout()
		if err != nil {
			return err
		}
		fmt.Println(i)
	case "gettimeleft":
		i, err := wd.TimeLeft()
		if err != nil {
			return err
		}
		fmt.Println(i)
	}
	return nil
}

func main() {
	if err := runCommand(); err != nil {
		log.Fatal(err)
	}
}
