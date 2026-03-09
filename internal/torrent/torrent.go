package torrent

import (
	"bytes"
	"crypto/sha1"
	"errors"

	"github.com/Inengs/bittorrent-client/internal/bencode"
	bencodelib "github.com/jackpal/bencode-go"
)

// the clean version the program will actually use
type TorrentFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

func ToTorrentFile(bto bencode.BencodeTorrent) (TorrentFile, error) {
	pieceHashes, err := splitPieces(bto.Info.Pieces)
	if err != nil {
		return TorrentFile{}, err
	}

	infoHash, err := calculateInfoHash(bto.Info)
	if err != nil {
		return TorrentFile{}, err
	}

	return TorrentFile{
		Announce: bto.Announce,
		InfoHash: infoHash,
		Name: bto.Info.Name,
		Length: bto.Info.Length,
		PieceLength: bto.Info.PieceLength,
		PieceHashes: pieceHashes,
	}, nil
}

func splitPieces(pieces string) ([][20]byte, error) {
	byteOfPieces := []byte(pieces)

	if len(byteOfPieces) % 20 != 0 {
		return nil, errors.New("length of byte array is supposed to be divisible by 20")
	}

	numPieces := len(byteOfPieces) / 20 // check the number of 20 byte pieces
	hashes := make([][20]byte, numPieces) // this creates an array of slices with numPieces slots

	for i := 0; i < numPieces; i++{
		copy(hashes[i][:], byteOfPieces[i*20: (i+1)*20])
	}

	return hashes, nil
}

func calculateInfoHash(info bencode.BencodeInfo) ([20]byte, error) {
	var infoBuffer bytes.Buffer

	err := bencodelib.Marshal(&infoBuffer, info)
	if err != nil {
		return [20]byte{}, err
	}

	hash := sha1.Sum(infoBuffer.Bytes())

	return hash, nil
}