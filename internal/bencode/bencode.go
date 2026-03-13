package bencode

import (
	"io"

	"github.com/jackpal/bencode-go"
)

// Deals with reading and parsing the .torrent file

// A .torrent file is just a bencode encoded dictionary and the info key contains another nested dictionary with the file details - name size, piece length, and the raw piece hashes.

// maps directly to the info dictionary of the .torrent file
type BencodeInfo struct {
	// each 20 bytes is a SHA1 hash of one piece
	Pieces string `bencode:"pieces"`
	// size of each piece in bytes
	PieceLength int `bencode:"piece length"`
	// total file size in bytes
	Length int `bencode:"length"`
	// filename
	Name string `bencode:"name"`
}

// the full raw .torrent file
type BencodeTorrent struct {
	// the tracker url
	Announce string `bencode:"announce"`
	// the nested info dictionary
	Info BencodeInfo `bencode:"info"`
}

func Open(r io.Reader) (*BencodeTorrent, error) {
	bT := new(BencodeTorrent) // lives on the heap, so it lives beyond this function

	err := bencode.Unmarshal(r, bT) // reads the raw bencode bytes from r(the torrent file) and fills in all the fields of bT automatically
	if err != nil {
		return nil, err
	}
	
	return bT, nil // this then returns the filled struct and nil error
}