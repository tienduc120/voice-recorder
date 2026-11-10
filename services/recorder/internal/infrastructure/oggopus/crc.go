package oggopus

const (
	// oggPolynomial is CRC-32 as Ogg uses it: not reflected, initial value 0 and no final XOR, so hash/crc32,
	// which is reflected, cannot compute it.
	oggPolynomial = 0x04c11db7
	topBit        = 0x80000000
	bitsPerByte   = 8
	byteShift     = 24
)

// CRC is the checksum of an Ogg page whose own checksum field is zero. Bitwise, because a page a second needs no
// table and a table would be mutable package state.
func CRC(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc ^= uint32(b) << byteShift
		for range bitsPerByte {
			if crc&topBit != 0 {
				crc = crc<<1 ^ oggPolynomial
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
