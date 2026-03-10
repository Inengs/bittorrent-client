package bitfield

type Bitfield []byte

func (bf Bitfield) HasPiece(index int) bool {
	byteIndex := index / 8
	bitOffset := index % 8

	// create a mask with only the bit we care about set
	shift := 7 - bitOffset
	mask := byte(1 << shift)

	// AND the byte with the mask
	result := bf[byteIndex] & mask

	// if result is not zero, the bit is set
	return result != 0
}

func (bf Bitfield) SetPiece(index int) {
	byteIndex := index / 8
	bitOffset := index % 8

	// create a mask with only the bit we want to set
	shift := 7 - bitOffset
	mask := byte(1 << shift)

	// OR the byte with the mask to set that specific bit to 1
	// without touching the other bits in the byte
	bf[byteIndex] |= mask
}