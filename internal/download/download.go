package download

import (
	"encoding/binary"
	"errors"
	"net"

	"github.com/Inengs/bittorrent-client/internal/bitfield"
	"github.com/Inengs/bittorrent-client/internal/peer"
	"github.com/Inengs/bittorrent-client/internal/torrent"
)

// this represents a piece to download
type PieceWork struct {
	Index  int
	Hash   [20]byte
	Length int
}

// represents a completed downloaded piece
type PieceResult struct {
	Index int
	Data  []byte
}

const BlockSize = 16834 // 16KB in bytes

func Download(t torrent.TorrentFile, peers []peer.Peer, peerID [20]byte) ([]byte, error) {
	pieceWork := make(chan PieceWork, len(t.PieceHashes))
	pieceResult := make(chan PieceResult)

    // queue up all pieces
    for i, hash := range t.PieceHashes {
        pieceWork <- PieceWork{
            Index:  i,
            Hash:   hash,
            Length: t.PieceLength,
        }
    }

    // launch a goroutine per peer
    for _, p := range peers {
        go func(p peer.Peer) {
            downloadFromPeer(p, peerID, t.InfoHash, pieceWork, pieceResult)
        }(p)
    }

	// collect from Peers
	buf := make([]byte, t.Length)
	donePieces := 0
	for donePieces < len(t.PieceHashes) {
		result := <- pieceResult
		begin := result.Index * t.PieceLength
		copy(buf[begin:], result.Data)
		donePieces++
	}

	close(pieceWork)
	return buf, nil
}

func downloadFromPeer(p peer.Peer, peerID [20]byte, infoHash [20]byte, pw chan PieceWork, pR chan PieceResult) error{
	conn, err := peer.Connect(p, infoHash, peerID)
	if err != nil {
		return err
	}
	defer conn.Close()

	// peer immediately sends bitfield after handshake
	msg, err := peer.ReadMessage(conn)
	if err != nil {
		return err
	}

	if msg.ID != peer.MsgBitfield {
		return errors.New("expected bitfield message")
	}

	bf := bitfield.Bitfield(msg.Payload)

	// send interested
    conn.Write((&peer.Message{ID: peer.MsgInterested}).Serialize())

 	// wait for unchoke
    for {
        msg, err := peer.ReadMessage(conn)
        if err != nil {
            return err
        }
        if msg.ID == peer.MsgUnchoke {
            break
        }
    }

	// download loop
	for work := range pw {
		if !bf.HasPiece(work.Index) {
			pw <- work // put back we dont have it 
			continue
		}

		data, err := downloadPiece(conn, work)
		if err != nil {
			pw <- work // put back on failure
			continue
		}

		pR <- PieceResult{Index: work.Index, Data: data}
	}

	return nil
}

func downloadPiece(conn net.Conn, work PieceWork) ([]byte, error) {
	buffer := make([]byte, work.Length) // create an empty buffer the exact size of the piece

	for i := 0; i < work.Length; i += BlockSize {
		begin := i // starting byte position within the piece 0, 16384, 32768
		requestLength := BlockSize // assuming we want a full 16kb block 

		remainingBytes := work.Length - begin
		weAreRequestingTooMuch := (begin + requestLength) > work.Length

		if weAreRequestingTooMuch {
			requestLength = remainingBytes
		}

		pieceIndex := work.Index

		// build and send the request
		payload := make([]byte, 12) // 3 fields * 4 bytes each
		binary.BigEndian.PutUint32(payload[0:4], uint32(pieceIndex)) // put index as first 4 bytes 
		binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
		binary.BigEndian.PutUint32(payload[8:12], uint32(requestLength))

	}

	return buffer, nil
}