package bitfield

type Bitfield []byte

// byte 0           byte 1           byte 2
// [0,1,2,3,4,5,6,7] [8,9,10,11,12,13,14,15] [16,17,18,19,20,21,22,23]

// byte 0          byte 1
// 7 6 5 4 3 2 1 0 | 7 6 5 4 3 2 1 0        - counting the bittorrent way

// the aim is that all bits need to be 1, that shows that the peer has every piece of the torrent

func (bf Bitfield) HasPiece(index int) bool {
	byteIndex := index / 8 // if index == 10, 10/8 = 1. , so it means that 10 lives in the second byte which is byte 1
	bitOffset := index % 8 // 10 % 8 == 2, then we know 10 is the second bit in byte 1

	// create a mask with only the bit we care about set
	shift := 7 - bitOffset   // this flips it, since 10 is at offset 2 from the left, we need to shift left by 5, because bits are counted left to right in BitTorrent
	mask := byte(1 << shift) // this creates a byte with only one bit set to 1, at the position 2 from the left, bear in mind that bytes are 0 based index

	// bounds check
	if byteIndex >= len(bf) {
		return false
	}

	// AND the byte with the mask
	result := bf[byteIndex] & mask // when you AND it with the actual byte you will notice that they are both set to 1 at the exact same index which gives 1

	// if result is not zero, the bit is set
	return result != 0 // if it is zero it means the bit is not set
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