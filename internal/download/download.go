package download

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
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

const BlockSize = 16384 // 16KB in bytes

func Download(t torrent.TorrentFile, peers []peer.Peer, peerID [20]byte) ([]byte, error) {
	if len(peers) == 0 {
		return nil, errors.New("no peers available")
	}

	fmt.Printf("Starting download with %d peers (max 5 concurrent)\n", len(peers))
	
	pieceWork := make(chan PieceWork, len(t.PieceHashes))
	pieceResult := make(chan PieceResult, len(t.PieceHashes)) // buffered

	// Queue all pieces
	for i, hash := range t.PieceHashes {
		length := t.PieceLength
		if i == len(t.PieceHashes)-1 {
			length = t.Length % t.PieceLength
			if length == 0 {
				length = t.PieceLength
			}
		}
		pieceWork <- PieceWork{Index: i, Hash: hash, Length: length}
	}

	close(pieceWork)   // ← CRITICAL: close it immediately after queuing

    // launch a goroutine per peer
    for _, p := range peers {
        go func(p peer.Peer) {
            err := downloadFromPeer(p, peerID, t.InfoHash, t, pieceWork, pieceResult)
			if err != nil {
            	fmt.Printf("peer %s failed: %v\n", p.IP, err)
        	}
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
		fmt.Printf("✓ Piece %d/%d downloaded\n", donePieces, len(t.PieceHashes))
	}

	return buf, nil
}

func runPeerSession(conn net.Conn, t torrent.TorrentFile, pw chan PieceWork, pR chan PieceResult) error {
	// peer immediately sends bitfield after handshake
	msg, err := peer.ReadMessage(conn)
	if err != nil {
		fmt.Printf("failed to read bitfield: %v\n", err)
		return err
	}

	var bf bitfield.Bitfield
	var alreadyUnchoked bool
	switch msg.ID {
	case peer.MsgBitfield:
		fmt.Println("got bitfield")
		bf = bitfield.Bitfield(msg.Payload)
	case peer.MsgUnchoke:
		fmt.Println("peer sent unchoke instead of bitfield, assuming all pieces available")
		bf = make(bitfield.Bitfield, len(t.PieceHashes)/8+1)
		for i := range bf {
			bf[i] = 0xff
		}
		alreadyUnchoked = true
	default:
		return fmt.Errorf("unexpected message ID: %d", msg.ID)
	}

	fmt.Println("got bitfield")

	// bf := bitfield.Bitfield(msg.Payload)

	// send interested
    conn.Write((&peer.Message{ID: peer.MsgInterested}).Serialize())
	fmt.Println("sent interested")

	if !alreadyUnchoked {	
		// wait for unchoke
		for {
			msg, err := peer.ReadMessage(conn)
			if err != nil {
				fmt.Printf("failed waiting for unchoke: %v\n", err)
				return err
			}
			if msg.ID == peer.MsgUnchoke {
				fmt.Println("got unchoke")
				break
			}

			// ignore have messages while waiting, which means i just finished downloading a piece
			if msg.ID == peer.MsgHave {
				continue
			}
			fmt.Printf("waiting for unchoke, got message ID: %d\n", msg.ID)
		}
	}
	// download loop
	for work := range pw {
		if !bf.HasPiece(work.Index) {
			// pw <- work // put back we dont have it 
			continue
		}

		data, err := downloadPiece(conn, work)
		if err != nil {
			fmt.Printf("Failed to download piece %d: %v\n", work.Index, err)
			// pw <- work // put back on failure
			continue
		}

		// verify the hash
		hash := sha1.Sum(data)
		if hash != work.Hash {
			fmt.Printf("Hash mismatch on piece %d\n", work.Index)
			// pw <- work // hash mismatch, requeue
			continue
		}

		pR <- PieceResult{Index: work.Index, Data: data}
	}

	return nil
}

func downloadFromPeer(p peer.Peer, peerID [20]byte, infoHash [20]byte, t torrent.TorrentFile, pw chan PieceWork, pR chan PieceResult) error{
	conn, err := peer.Connect(p, infoHash, peerID)
	if err != nil {
		fmt.Printf("failed to connect to peer %s: %v\n", p.IP, err)
		return err
	}
	defer conn.Close()

	return runPeerSession(conn, t, pw, pR)
}

// this function sends a request to the peer, waits for a piece message, extracts the data block, stores it in a buffer, returns the full piece
func downloadPiece(conn net.Conn, work PieceWork) ([]byte, error) {
	buffer := make([]byte, work.Length) // create an empty buffer the exact size of the piece

	for offset := 0; offset < work.Length; offset += BlockSize {
		// begin := i // starting byte position within the piece 0, 16384, 32768
		requestLength := BlockSize // assuming we want a full 16kb block 

		remainingBytes := work.Length - offset
		weAreRequestingTooMuch := (offset + requestLength) > work.Length

		if weAreRequestingTooMuch {
			requestLength = remainingBytes
		}

		pieceIndex := work.Index

		// build and send the request
		payload := make([]byte, 12) // 3 fields * 4 bytes each
		binary.BigEndian.PutUint32(payload[0:4], uint32(pieceIndex)) // put index as first 4 bytes 
		binary.BigEndian.PutUint32(payload[4:8], uint32(offset))
		binary.BigEndian.PutUint32(payload[8:12], uint32(requestLength))

		// send the request message
		msg := &peer.Message{
			ID: peer.MsgRequest,
			Payload: payload,
		}

		_, err := conn.Write(msg.Serialize())
		if err != nil {
			return nil, err
		}

		// Read until we get the piece we want
		for {
			msg, err := peer.ReadMessage(conn)
			if err != nil {
				return nil, err
			}
			if msg.ID == peer.MsgPiece {
				if len(msg.Payload) < 8 {
					continue
				}
				index := int(binary.BigEndian.Uint32(msg.Payload[0:4]))
				begin := int(binary.BigEndian.Uint32(msg.Payload[4:8]))
				block := msg.Payload[8:]

				// Check if this is the block we asked for
				if index == work.Index && begin == offset {
					copy(buffer[begin:], block)
					break // got what we wanted, move to next block
				}
			} else if msg.ID == peer.MsgChoke {
				return nil, errors.New("peer choked us")
			} else if msg.ID != 0 {
				fmt.Printf("Ignored message ID %d\n", msg.ID)
			}
		}
	}
	return buffer, nil
}