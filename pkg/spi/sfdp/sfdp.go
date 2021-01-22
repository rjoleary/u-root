// Useful references:
// * Linux: Documentation/spi/spidev
// *
package sfdp

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	SFDPBasicTableId                   = 0x00
	SFDPBasicTable4KiBEraseOpcodeDword = 0
	SFDPBasicTableDensityDword         = 1
)

type SFDP struct {
	SFDPHeader
	Parameters []SFDPParameter
}

type SFDPHeader struct {
	// Signature is 0x50444653 ("SFDP") if the chip supports SFDP.
	Signature                uint32
	MinorRev                 uint8
	MajorRev                 uint8
	NumberOfParameterHeaders uint8
	_                        uint8
}

type SFDPParameter struct {
	SFDPParameterHeader
	// Id is IdMSB:IdLSB.
	Id    uint16
	Table []uint32
}

type SFDPParameterHeader struct {
	IdLSB    uint8
	MinorRev uint8
	MajorRev uint8
	// Length is in dwords.
	Length uint8
	// The top byte of Pointer is the MSB of the Id for revision 1.5.
	Pointer uint32
}

type ReaderAt interface {
	SFDPReadAt(offset uint32, out []byte) error
}

// Buffer for holding an SFDP to be parsed. Primarily used for testing.
type Buffer []byte

// SFDPReadAt implements sfdp.ReaderAt for Buffer.
func (b Buffer) SFDPReadAt(offset uint32, out []byte) error {
	offset = offset & 0x00ffffff
	if int(offset)+len(out) > len(b) {
		return fmt.Errorf("invalid offset")
	}
	copy(out, b[offset:])
	return nil
}

func ParseSFDP(r ReaderAt) (*SFDP, error) {
	headerBuf := make([]byte, binary.Size(SFDPHeader{}))
	if err := r.SFDPReadAt(0, headerBuf); err != nil {
		return nil, err
	}
	if string(headerBuf[:4]) != "SFDP" {
		return nil, fmt.Errorf("chip does not support SFDP")
	}
	var header SFDPHeader
	if err := binary.Read(bytes.NewBuffer(headerBuf), binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	parametersBuf := make([]byte, binary.Size(SFDPParameterHeader{})*int(header.NumberOfParameterHeaders))
	if err := r.SFDPReadAt(uint32(binary.Size(SFDPHeader{})), parametersBuf); err != nil {
		return nil, err
	}
	sfdp := &SFDP{
		SFDPHeader: header,
		Parameters: make([]SFDPParameter, header.NumberOfParameterHeaders),
	}
	for i := range sfdp.Parameters {
		p := &sfdp.Parameters[i]
		if err := binary.Read(bytes.NewBuffer(parametersBuf), binary.LittleEndian, p); err != nil {
			return nil, err
		}
		p.Id = uint16(p.IdLSB)
		if p.MajorRev == 1 && p.MinorRev == 5 {
			p.Id |= uint16((p.Pointer >> 16) & 0xff00)
		}
		tableBuf := make([]byte, int(p.Length)*4)
		if err := r.SFDPReadAt(p.Pointer, tableBuf); err != nil {
			return nil, err
		}
		if err := binary.Read(bytes.NewBuffer(tableBuf), binary.LittleEndian, &p.Table); err != nil {
			return nil, err
		}
	}
	return sfdp, nil
}

// TableDword reads a dword from the SFDP table with the given id.
func (s *SFDP) TableDword(id uint16, dword int) (uint32, error) {
	for _, p := range s.Parameters {
		if p.Id == id {
			if dword > len(p.Table) {
				return 0, fmt.Errorf("out of range")
			}
			return p.Table[dword], nil
		}
	}
	return 0, fmt.Errorf("out of range")
}

func (s *SFDP) Size() (int, error) {
	size, err := s.TableDword(SFDPBasicTableId, SFDPBasicTableDensityDword)
	if err != nil {
		return -1, err
	}
	if size&0x80000000 != 0 {
		return -1, fmt.Errorf("chip >= 2GiB")
	}
	return int(size), nil
}

func (s *SFDP) Erase4KiBOpcode() (uint8, error) {
	dword, err := s.TableDword(SFDPBasicTableId, SFDPBasicTable4KiBEraseOpcodeDword)
	if err != nil {
		return 0xff, err
	}
	opcode := uint8((dword >> 8) & 0xff)
	if opcode == 0xff {
		return 0xff, fmt.Errorf("no erase opcode")
	}
	return opcode, nil
}
