// Useful references:
// * Linux: Documentation/spi/spidev
// *
package flash

import (
	"encoding/binary"

	"github.com/u-root/u-root/pkg/spi"
	"github.com/u-root/u-root/pkg/spi/sfdp"
)

// Flash provides operations for SPI flash chips.
type Flash struct {
	SPI *spi.SPI

	// Cache SFDP.
	sfdp    *sfdp.SFDP
	sfdpErr error
}

// SFDPReadAt reads from the given offset in the SFDP address space. The most
// significant byte of offset is ignored.
func (f *Flash) SFDPReadAt(offset uint32, out []byte) error {
	tx := []byte{
		0x5a,             // command
		0x00, 0x00, 0x00, // offset, 3-bytes, little-endian
		0xff, // dummy 0xff
	}
	binary.LittleEndian.PutUint32(tx[1:], offset&0xff000000)
	return f.SPI.WriteThenRead(tx, out)
}

// SFDP reads, parses and returns the SFDP from the flash chip. The value is
// cached.
func (f *Flash) SFDP() (*sfdp.SFDP, error) {
	if f.sfdpErr != nil {
		return nil, f.sfdpErr
	}
	if f.sfdp == nil {
		f.sfdp, f.sfdpErr = sfdp.ParseSFDP(f)
	}
	return f.sfdp, f.sfdpErr
}
