package spi

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// See Linux "include/uapi/linux/spi/spidev.h" and
// "Documentation/spi/spidev.rst"

// Various ioctl numbers.
const (
	iocRdMode        = 0x80016b01
	iocWrMode        = 0x40016b01
	iocRdLSBFirst    = 0x80016b02
	iocWrLSBFirst    = 0x40016b02
	iocRdBitsPerWord = 0x80016b03
	iocWrBitsPerWord = 0x40016b03
	iocRdMaxSpeedHz  = 0x80046b04
	iocWrMaxSpeedHz  = 0x40046b04
	iocRdMode32      = 0x80046b05
	iocWrMode32      = 0x40046b05
)

// iocMessage is an ioctl number for n Transfers.
func iocMessage(n int) uint32 {
	const (
		sizeBits  = 14
		sizeShift = 16
	)
	size := uint32(n * binary.Size(Transfer{}))
	if n < 0 || size > (1<<sizeBits) {
		return iocMessage(0)
	}
	return 0x40006b00 | (size << sizeShift)
}

type Mode uint32

const (
	CPHA Mode = 1 << iota
	CPOL
	CS_HIGH
	LSB_FIRST
	THREE_WIRE
	LOOP
	NO_CS
	READY
	TX_DUAL
	TX_QUAD
	RX_DUAL
	RX_QUAD
)

// iocTransfer is the data type used by the iocMessage ioctl. Multiple such
// transfers may be chained together in a single ioctl call.
type iocTransfer struct {
	TxBuf          uint64
	RxBuf          uint64
	Length         uint32
	SpeedHz        uint32
	DelayUsecs     uint16
	BitsPerWord    uint8
	CSChange       uint8
	TxNBits        uint8
	RxNBits        uint8
	WordDelayUsecs uint8
	Pad            uint8
}

type Transfer struct {
	Tx             []byte
	Rx             []byte
	SpeedHz        uint32
	DelayUsecs     uint16
	BitsPerWord    uint8
	CSChange       bool
	TxNBits        uint8
	RxNBits        uint8
	WordDelayUsecs uint8
}

// SPI performs low-level SPI operations.
type SPI struct {
	f *os.File
}

// OpenSPI opens a new SPI device. dev is a filename such as "/dev/spidev0.0".
// Remember to call Close().
func OpenSPI(dev string) (*SPI, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &SPI{f: f}, err
}

// Close closes SPI.
func (s *SPI) Close() error {
	return s.f.Close()
}

// Read reads from SPI, half-duplex. All writes are zero.
func (s *SPI) Read(buf []byte) (int, error) {
	return s.f.Read(buf)
}

// Write writes to SPI, half-duplex.
func (s *SPI) Write(buf []byte) (int, error) {
	return s.f.Write(buf)
}

// WriteThenRead performs a Write, then a Read while maintaining the chip
// select asserted.
func (s *SPI) WriteThenRead(tx []byte, rx []byte) error {
	return s.Transfer([]Transfer{
		{Tx: tx, CSChange: false},
		{Rx: rx, CSChange: true},
	})
}

func (s *SPI) Transfer(transfers []Transfer) error {
	// Copy data into unmanaged buffer because the garbage collector may move
	// pointers at any time.
	var bufSize = 0
	for _, t := range transfers {
		if len(t.Tx) != len(t.Rx) && (len(t.Tx) == 0) == (len(t.Rx) == 0) {
			return fmt.Errorf("rx/tx lengths must equal, or one length is zero")
		}
		bufSize += len(t.Tx) + len(t.Rx)
	}
	buf, err := unix.Mmap(-1, 0, bufSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		return err
	}
	defer unix.Munmap(buf)

	var it []iocTransfer
	var bufOffset = 0
	for _, t := range transfers {
		var csChange uint8
		if t.CSChange {
			csChange = 1
		}
		copy(buf[bufOffset:], t.Tx)
		copy(buf[bufOffset+len(t.Tx):], t.Rx)
		length := len(t.Tx)
		if length == 0 {
			length = len(t.Rx)
		}
		it = append(it, iocTransfer{
			TxBuf:          uint64(uintptr(unsafe.Pointer(&buf[bufOffset]))),
			RxBuf:          uint64(uintptr(unsafe.Pointer(&buf[bufOffset+len(t.Tx)]))),
			Length:         uint32(length),
			SpeedHz:        t.SpeedHz,
			DelayUsecs:     t.DelayUsecs,
			BitsPerWord:    t.BitsPerWord,
			CSChange:       csChange,
			TxNBits:        t.TxNBits,
			RxNBits:        t.RxNBits,
			WordDelayUsecs: t.WordDelayUsecs,
		})
		if len(t.Tx) == 0 {
			it[len(it)-1].TxBuf = 0
		}
		if len(t.Rx) == 0 {
			it[len(it)-1].RxBuf = 0
		}
		bufOffset += len(t.Tx) + len(t.Rx)
	}

	if _, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(),
		uintptr(iocMessage(len(transfers))),
		uintptr(unsafe.Pointer(&it[0]))); err != 0 {
		return err
	}

	// Copy out rx.
	bufOffset = 0
	for _, t := range transfers {
		copy(t.Rx, buf[bufOffset+len(t.Tx):])
		bufOffset += len(t.Tx) + len(t.Rx)
	}

	return nil
}

func (s *SPI) Mode() (Mode, error) {
	var m Mode
	_, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(), iocRdMode32, uintptr(unsafe.Pointer(&m)))
	return m, err
}

func (s *SPI) SetMode(m Mode) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(), iocWrMode32, uintptr(unsafe.Pointer(&m)))
	return err
}

func (s *SPI) LSBFirst() (bool, error) {
	var v uint8
	_, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(), iocRdLSBFirst, uintptr(unsafe.Pointer(&v)))
	return v != 0, err
}

func (s *SPI) SetLSBFirst(lsbFirst bool) error {
	var v uint
	if lsbFirst {
		v = 1
	}
	_, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(), iocWrLSBFirst, uintptr(unsafe.Pointer(&v)))
	return err
}

func (s *SPI) BitsPerWord() (uint32, error) {
	var bpw uint32
	_, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(), iocRdBitsPerWord, uintptr(unsafe.Pointer(&bpw)))
	return bpw, err
}

func (s *SPI) SetBitsPerWord(bpw uint32) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(), iocWrBitsPerWord, uintptr(unsafe.Pointer(&bpw)))
	return err
}

// GetSpeedHz gets the transfer speed.
func (s *SPI) SpeedHz() (uint32, error) {
	var hz uint32
	_, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(), iocRdMaxSpeedHz, uintptr(unsafe.Pointer(&hz)))
	return hz, err
}

// SetSpeedHz sets the transfer speed.
func (s *SPI) SetSpeedHz(hz uint32) error {
	_, _, err := unix.Syscall(unix.SYS_IOCTL, s.f.Fd(), iocWrMaxSpeedHz, uintptr(unsafe.Pointer(&hz)))
	return err
}
