// Copyright 2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package watchdog provides functions for interacting with the Linux watchdog.
//
// The basic usage is:
//     wd, err := watchdog.Open(watchdog.Dev)
//     if err != nil { ... }
//     while running {
//         wd.KeepAlive()
//     }
//     wd.MagicClose()
//
// Open() arms the watchdog. MagicClose() disarms the watchdog.
//
// Alternatively, use Close() which has behavior dependent on
// CONFIG_WATCHDOG_NOWAYOUT.
//
// Note not every watchdog driver supports every function!
//
// For more, see:
// https://www.kernel.org/doc/Documentation/watchdog/watchdog-api.txt
package watchdog

import (
	"log"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Dev is the name of the first watchdog. If there are multiple watchdogs, they
// are named /dev/watchdog0, /dev/watchdog1, ...
const Dev = "/dev/watchdog"

// Various ioctl numbers.
const (
	wdiocGetSupport    = 0x80285700
	wdiocGetStatus     = 0x80045701
	wdiocGetBootStatus = 0x80045702
	wdiocGetTemp       = 0x80045703
	wdiocSetOptions    = 0x80045704
	wdiocKeepAlive     = 0x80045705
	wdiocSetTimeout    = 0xc0045706
	wdiocGetTimeout    = 0x80045707
	wdiocSetPreTimeout = 0xc0045708
	wdiocGetPreTimeout = 0x80045709
	wdiocGetTimeLeft   = 0x8004570a
)

// Status contains flags returned by Status() and BootStatus(). These are the
// same flags used for Support()'s options field.
type Status int32

// Bitset of possible flags for the Status() type.
const (
	// Unknown flag error
	StatusUnknown Status = -1
	// Reset due to CPU overheat
	StatusOverheat Status = 0x0001
	// Fan failed
	StatusFanFault Status = 0x0002
	// External relay 1
	StatusExtern1 Status = 0x0004
	// ExStatusl relay 2
	StatusExtern2 Status = 0x0008
	// Power bad/power fault
	StatusPowerUnder Status = 0x0010
	// Card previously reset the CPU
	StatusCardReset Status = 0x0020
	// Power over voltage
	StatusPowerOver Status = 0x0040
	// Set timeout (in seconds)
	StatusSetTimeout Status = 0x0080
	// Supports magic close char
	StatusMagicClose Status = 0x0100
	// Pretimeout (in seconds), get/set
	StatusPreTimeout Status = 0x0200
	// Watchdog triggers a management or other external alarm not a reboot
	StatusAlarmOnly Status = 0x0400
	// Keep alive ping reply
	StatusKeepAlivePing Status = 0x8000
)

// Option are options passed to SetOptions().
type Option int32

// Bitset of possible flags for the Option type.
const (
	// Unknown status error
	OptionUnknown Option = -1
	// Turn off the watchdog timer
	OptionDisableCard Option = 0x0001
	// Turn on the watchdog timer
	OptionEnableCard Option = 0x0002
	// Kernel panic on temperature trip
	OptionTempPanic Option = 0x0004
)

type Watchdog struct {
	f *os.File
}

func Open(dev string) (*Watchdog, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &Watchdog{f: f}, nil
}

// Close has behavior dependent on CONFIG_WATCHDOG_NOWAYOUT.
func (w *Watchdog) Close() error {
	return w.f.Close()
}

// MagicClose disarms the watchdog.
func (w *Watchdog) MagicClose() error {
	if _, err := w.f.Write([]byte("V")); err != nil {
		w.f.Close()
		return err
	}
	return w.f.Close()
}

// Support returns the WatchdogInfo struct.
func (w *Watchdog) Support() (*unix.WatchdogInfo, error) {
	var wi unix.WatchdogInfo
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocGetSupport, uintptr(unsafe.Pointer(&wi))); err != 0 {
		return wi, err
	}
	return &wi, nil
}

// Status returns the current status.
func (w *Watchdog) Status() (Status, error) {
	var flags int32
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocGetStatus, uintptr(unsafe.Pointer(&flags))); err != 0 {
		return StatusUnknown, err
	}
	return Status(flags), nil
}

// BootStatus returns the status at the last reboot.
func (w *Watchdog) BootStatus() (Status, error) {
	var flags int32
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocGetBootStatus, uintptr(unsafe.Pointer(&flags))); err != 0 {
		return StatusUnknown, err
	}
	return Status(flags), nil
}

// Temp returns temperature in degrees Fahrenheit.
func (w *Watchdog) Temp() (int32, error) {
	var temp int32
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocGetTemp, uintptr(unsafe.Pointer(&temp))); err != 0 {
		return 0, err
	}
	return temp, nil
}

// SetOptions can be used to control some aspects of the cards operation.
func (w *Watchdog) SetOptions(options Option) error {
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocSetOptions, uintptr(unsafe.Pointer(&options))); err != 0 {
		return err
	}
	return nil
}

// KeepAlive pets the watchdog.
func (w *Watchdog) KeepAlive() error {
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocKeepAlive, 0); err != 0 {
		return err
	}
	return nil
}

// SetTimeout sets the watchdog timeout on the fly. It returns the real timeout
// used and this timeout might differe from the requested one due to limitation
// of the hardware.
func (w *Watchdog) SetTimeout(timeout int32) (int32, error) {
	originalTimeout := timeout
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocSetTimeout, uintptr(unsafe.Pointer(&timeout))); err != 0 {
		return 0, err
	}
	if originalTimeout != timeout {
		log.Printf("Watchdog timeout set to %ds, wanted %ds", timeout, originalTimeout)
	}
	return timeout, nil
}

// Timeout returns the current watchdog timeout.
func (w *Watchdog) Timeout() (int32, error) {
	var timeout int32
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocGetTimeout, uintptr(unsafe.Pointer(&timeout))); err != 0 {
		return 0, err
	}
	return timeout, nil
}

// SetPreTimeout sets the watchdog pretimeout on the fly. The pretimeout is the
// number of seconds before the timeout before triggering the preaction (such
// as an NMI, interrupt, ...)
func (w *Watchdog) SetPreTimeout(timeout int32) (int32, error) {
	originalTimeout := timeout
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocSetPreTimeout, uintptr(unsafe.Pointer(&timeout))); err != 0 {
		return 0, err
	}
	if originalTimeout != timeout {
		log.Printf("Watchdog pretimeout set to %ds, wanted %ds", timeout, originalTimeout)
	}
	return timeout, nil
}

// PreTimeout returns the current watchdog pretimeout.
func (w *Watchdog) PreTimeout() (int32, error) {
	var timeout int32
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocGetPreTimeout, uintptr(unsafe.Pointer(&timeout))); err != 0 {
		return 0, err
	}
	return timeout, nil
}

// TimeLeft returns the number of seconds before the reboot.
func (w *Watchdog) TimeLeft() (int32, error) {
	var left int32
	if _, _, err := unix.Syscall(unix.SYS_IOCTL, w.f.Fd(), wdiocGetTimeLeft, uintptr(unsafe.Pointer(&left))); err != 0 {
		return 0, err
	}
	return left, nil
}
