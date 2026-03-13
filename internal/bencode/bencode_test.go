package bencode

import (
	"strings"
	"testing"
)

// sample bencode encoded string
// d8:announce35:http://tracker.example.com/announce4:infod6:lengthi500e4:name10:sample.txt12:piece lengthi262144e6:pieces20:xxxxxxxxxxxxxxxxxxxx ee

// d...e — dictionary
// 8:announce — key "announce" (8 chars)
// 35:http://tracker.example.com/announce — the tracker URL (35 chars)
// 4:info — key "info"
// d...e — nested dictionary
// 6:length i500e — file size 500 bytes
// 4:name 10:sample.txt — filename
// 12:piece length i262144e — piece size
// 6:pieces 20:xxxxxxxxxxxxxxxxxxxx — 20 bytes of fake piece hashes (one piece)

func TestOpen(t *testing.T) {
	input := "d8:announce35:http://tracker.example.com/announce4:infod6:lengthi500e4:name10:sample.txt12:piece lengthi262144e6:pieces20:xxxxxxxxxxxxxxxxxxxxee"
    
	r := strings.NewReader(input) // make it an io.Reader so it satisfies the io.Reader interface

	bto, err := Open(r)
	if err != nil {
		t.Fatal(err) // this stops the whole test totally
	}

	if bto.Announce != "http://tracker.example.com/announce" {
        t.Errorf("expected announce URL got %s", bto.Announce)
    }
	
	if bto.Info.Name != "sample.txt" {
        t.Errorf("expected name sample.txt got %s", bto.Info.Name)
    }
}