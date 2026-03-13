package bitfield

import "testing"

// 1 means piece exists, peer has it, HasPiece returns true
// 0 means piece doesnt exist, peer doesnt have it, HasPiece returns false
func TestHasPiece(t *testing.T) {
	bf := Bitfield{0b10110100, 0b01101100} // binary literal of 180

	if bf.HasPiece(0) == false {
		t.Error("expected piece 0 to be present") // at index 0 the value is 1, and it is supposed to return true because 1 means it is set
	}

	if bf.HasPiece(1) == true {
		t.Error("expected piece 1 to be absent") // at index 1 the value is 0, this means the value is not set so it should return false
	}

	if bf.HasPiece(7) != false {
		t.Error("expected piece 7 to be absent")
	}

	if bf.HasPiece(8) != false {
		t.Error("expected piece 8 to be absent")
	}

	if bf.HasPiece(10) != true {
		t.Error("expected piece 7 to be present")
	}
}

func TestHasPieceEmptyBitfield(t *testing.T) {
	bf := Bitfield{}
	
	if bf.HasPiece(0) {
		t.Error("there should be no such index")
	}
}

func TestHasPieceAllSet(t *testing.T) {
	bf := Bitfield{0xff, 0xff}

	for i := 0; i < 16; i++ {
		if bf.HasPiece(i) == false {
			t.Errorf("expected piece %d to be present", i)
		}
	}
}

func TestHasPieceAllUnset(t *testing.T) {
	bf := Bitfield{0x00, 0x00}

	for i := 0; i < 16; i++ {
		if bf.HasPiece(i) == true {
			t.Errorf("expected piece %d to be absent", i)
		}
	}
}

func TestSetPiece(t *testing.T) {
	bf := Bitfield{0b00000000}

	bf.SetPiece(2)
	if !bf.HasPiece(2) {
		t.Error("expected piece 2 to be set after SetPiece")
	}
}

func TestSetPieceThenHasPiece(t *testing.T) {
	bf := Bitfield{0b10110100}

	bf.SetPiece(1)
	if bf.HasPiece(1) == false {
		t.Error("expected piece 1 to be set after setpiece")
	}
}