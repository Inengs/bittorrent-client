package download

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Inengs/bittorrent-client/internal/peer"
	"github.com/Inengs/bittorrent-client/internal/torrent"
)

const BlockSize = 16384 // 16KB

// PieceWork = one piece we need to download
type PieceWork struct {
	Index  int
	Hash   [20]byte
	Length int
}

// PieceResult = successfully downloaded and verified piece
type PieceResult struct {
	Index int
	Data  []byte
}

// Download is the main function you call from main.go
func Download(t torrent.TorrentFile, peers []peer.Peer, peerID [20]byte) ([]byte, error) {
	if len(peers) == 0 {
		return nil, errors.New("no peers available")
	}

	fmt.Printf("Starting download using %d peers...\n", len(peers))

	// these are buffered channels, it has a queue, the sender can keep pushing until the queue is full (the number of pieces is the size of the queue)- this is to avoid deadlock
	workChannel := make(chan PieceWork, len(t.PieceHashes))
	resultChannel := make(chan PieceResult, len(t.PieceHashes))

	// put all the pieces into the work queue
	for i, hash := range t.PieceHashes {
		length := t.PieceLength
		if i == len(t.PieceHashes)-1 {
			length = t.Length % t.PieceLength
			if length == 0 {
				length = t.PieceLength
			}
		}

		workChannel <- PieceWork{Index: i, Hash: hash, Length: length}
	}
	close(workChannel) // signals the worker that no more work is coming

	// Start worker goroutines (limited to max 5 concurrent peers)
	var wg sync.WaitGroup
	maxWorkers := 2
	if len(peers) < maxWorkers {
		maxWorkers = len(peers) // if the last remaining peers is less than 5, just start them
	}

	// each worker runs as a goroutine
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1) // registers the worker
		go func(workerID int) {
			defer wg.Done() // marks complete when it returns
			worker(t, peers, peerID, workChannel, resultChannel, workerID)
		}(i)
	}

	// collect all results
	fileData := make([]byte, t.Length)
	recieved := 0

	for recieved < len(t.PieceHashes) {
		result := <-resultChannel
		start := result.Index * t.PieceLength
		copy(fileData[start:], result.Data)
		recieved++
		fmt.Printf("✓ Piece %d/%d completed\n", recieved, len(t.PieceHashes))
	}

	wg.Wait() // wait for all workers to finish
	return fileData, nil
}

func worker(t torrent.TorrentFile, peers []peer.Peer, peerID [20]byte, workChannel <-chan PieceWork, resultChannel chan<- PieceResult, id int) {
	// workChannel - read-only channel(pulls piece jobs from), resultChannel - write only channel(send completed pieces from), id - work identifier for logging

	// pull work from Channel
	for work := range workChannel {
		success := false

		// try different peers until one succeeds
		for _, p := range peers {
			err := downloadFromPeer(p, peerID, t, work, resultChannel)
			if err == nil {
				success = true
				break
			}

			fmt.Printf("Worker %d: peer %s failed piece %d: %v\n", id, p.IP, work.Index, err)
		}

		if !success {
			// If all peers fail, we can re-queue or fail. For now we just skip (you can improve later)
			fmt.Printf("Failed to download piece %d from any peer\n", work.Index)
		}
	}
}
// downloadFromPeer connects, does handshake, and tries to download one piece
func downloadFromPeer(p peer.Peer, peerID [20]byte, t torrent.TorrentFile, work PieceWork, resultChannel chan<- PieceResult) error {
	// Takes the peer to connect to, your own peer ID, the torrent metadata, the piece to download, and a write-only channel to send the result back on once done.
	
	// Opens a TCP connection to the peer and performs the initial BitTorrent handshake — exchanging InfoHash and peer IDs to confirm you're both talking about the same torrent.
	conn, err := peer.Connect(p, t.InfoHash, peerID)
	if err != nil {
		return err
	}

	defer conn.Close() // Ensures the TCP connection is always closed when this function returns

	// simple Handshake flow
	if err := doHandshakeFlow(conn); err != nil {
    	return err
	}

	// download one piece
	data, err := downloadPiece(conn, work)
	if err != nil {
		return err
	}

	// verify Hash,  After downloading, you hash the data you received and compare it against the expected hash. If they don't match, the data is corrupted or the peer sent you bad data
	if sha1.Sum(data) != work.Hash {
		return errors.New("hash mismatch")
	}

	resultChannel <- PieceResult{Index: work.Index, Data: data} // push the verified piece into the results channel if everything works out, this is a write channel btw
	return nil
}

// do the interested + unchoke part
func doHandshakeFlow(conn net.Conn) error {
	// takes the TCP connection and the torrent metadata and returns an error

	// Read first message (bitfield or unchoke)
	msg, err := peer.ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("failed to read first message: %w", err)
	}

	alreadyUnchoked := false

	switch msg.ID {
	case peer.MsgBitfield:
		fmt.Println("Received bitfield") // means we have it complete
	case peer.MsgUnchoke:
		fmt.Println("Peer sent unchoke immediately (seed behavior)") // sometimes it just sends unchoke immediately
		alreadyUnchoked = true   // ← This was missing!
	default:
		fmt.Printf("Received unexpected message ID: %d after handshake\n", msg.ID)
	}

	// send interested, so the peer can unchoke me
	_, err = conn.Write((&peer.Message{ID: peer.MsgInterested}).Serialize())
	if err != nil {
		return err
	}

	// Only wait for unchoke if we didn't receive it already
	if !alreadyUnchoked {
		// Wait for unchoke
		for {
			msg, err = peer.ReadMessage(conn)
			if err != nil {
				return fmt.Errorf("failed waiting for unchoke: %w", err)
			}

			if msg.ID == peer.MsgUnchoke {
				fmt.Println("Received unchoke - ready to download")
				return nil
			}

			// Ignore keep-alives and other messages (like Have)
			if msg.ID != 0 {
				fmt.Printf("Ignored message ID %d while waiting for unchoke\n", msg.ID)
			}
		}
	} else {
		fmt.Println("Already unchoked - ready to download")
	}

	return nil
}

func downloadPiece(conn net.Conn, work PieceWork) ([]byte, error) {
	// connects via TCP to a peer and a PieceWork(this carries the piece index, length and hash), it then returns the raw bytes or an error
	data := make([]byte, work.Length) // create a buffer to store the downloaded block
	const MaxPendingRequests = 10

	offset := 0; 

	for offset < work.Length {
		// Step 1: Send up to MaxPendingRequests
		pending := 0

		for pending < MaxPendingRequests && offset < work.Length {
			// loop through the piece after every 16kb
			reqLen := BlockSize
			if offset+reqLen > work.Length {
				reqLen = work.Length - offset // this is to catch the last block, which might be smaller than 16kb
			}

			// Send request
			// The BitTorrent protocol requires a 12-byte request message with 3 fields packed in big-endian order — piece index, byte offset within the piece, and how many bytes you want.
			payload := make([]byte, 12)
			binary.BigEndian.PutUint32(payload[0:4], uint32(work.Index))
			binary.BigEndian.PutUint32(payload[4:8], uint32(offset))
			binary.BigEndian.PutUint32(payload[8:12], uint32(reqLen))

			_, err := conn.Write((&peer.Message{ID: peer.MsgRequest, Payload: payload}).Serialize()) // wraps the payload in a Message struct with type MsgRequest and sends it over TCP connectiont to the peer.
			if err != nil {
				return nil, fmt.Errorf("failed to send request: %w", err)
			}

			pending++
			offset += reqLen
		}

		// Read response, starts an inner loop that keeps reading messages from the peer until i get the response block
		// Step 2: Read responses until all pending requests are fulfilled
		for pending > 0 {
			msg, err := peer.ReadMessage(conn)
			if err != nil {
				return nil, fmt.Errorf("failed to read message: %w", err)
			}

			if msg.ID == peer.MsgPiece {
				// we have found the response piece
				if len(msg.Payload) < 8 {
					continue
				}

				// MsgPiece response has a 12-byte header, first 4 bytes piece index, second 4 bytes block offset, remaining bytes is the actual response
				index := int(binary.BigEndian.Uint32(msg.Payload[0:4]))
				begin := int(binary.BigEndian.Uint32(msg.Payload[4:8]))
				block := msg.Payload[8:]

				// this confirms that this is actually the response i requested for, if it is it copies it into the data and breaks out of the loop
				if index == work.Index {
					copy(data[begin:], block)
				}
				pending-- // one request fulfilled
			} else if msg.ID == peer.MsgChoke {
				// 
				return nil, errors.New("peer choked")
			}
		}
	}

	return data, nil
}