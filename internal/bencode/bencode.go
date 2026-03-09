package bencode

import (
	"io"

	"github.com/jackpal/bencode-go"
)

// Deals with reading and parsing the .torrent file

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
	Announce string `bencode:"announce"`
	Info BencodeInfo `bencode:"info"`
}

func Open(r io.Reader) (*BencodeTorrent, error) {
	bT := new(BencodeTorrent)

	err := bencode.Unmarshal(r, bT)
	if err != nil {
		return nil, err
	}
	
	return bT, nil
}