package download

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Inengs/bittorrent-client/internal/peer"
	"github.com/Inengs/bittorrent-client/internal/torrent"
)

// we need a mock net_conn interface to test the function downloadPiece

type mockConn struct {
	readBuffer []byte // this represents the data the peer sends to us
	writeBuffer []byte // represents the data our client sends to the peer
}

func (m *mockConn) Read(b []byte) (int, error) {
	if len(m.readBuffer) == 0 {
		return 0, io.EOF
	}
	// copies readbuffer into b, returns the number of bytes read and no error
	n := copy(b, m.readBuffer)
	m.readBuffer = m.readBuffer[n:]
	return n, nil
}

func (m *mockConn) Write(b []byte) (int, error) {
	// saves the written data into writeBuffer, return the number of bytes written
	m.writeBuffer = append(m.writeBuffer, b...)
	return len(b), nil
}

func (m *mockConn) Close() error { 
	// does nothing, real connections must be closed
	return nil 
}

func (m *mockConn) LocalAddr() net.Addr { return nil } // real connections have a local address, we just returned nil sha.
func (m *mockConn) RemoteAddr() net.Addr { return nil } // real connections have a remote address, we just returned nil

func (m *mockConn) SetDeadline(t time.Time) error { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// wll the above methods are written because we must implement every method in net.conn

type failConn struct{
	mockConn
}

func (f *failConn) Write(b []byte) (int, error) {
	return 0, errors.New("write failed")
}

var (
    // fakePeerID   [20]byte
    // fakeInfoHash [20]byte
    // fakePeer     = peer.Peer{IP: net.IP{127, 0, 0, 1}, Port: 6881}
    fakeTorrent  = torrent.TorrentFile{
        PieceHashes: [][20]byte{sha1.Sum(make([]byte, 1000))},
        PieceLength: 1000,
        Length:      1000,
    }
)

func buildPieceMessage(index int, begin int, data []byte) []byte {
	payload := make([]byte, 8+len(data))

	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	copy(payload[8:], data)

	msg := peer.Message{
		ID:      peer.MsgPiece,
		Payload: payload,
	}

	return msg.Serialize()
}

func Test_DownloadPiece_SingleBlock(t *testing.T) {
	work := PieceWork{
		// this simulates a piece that needs to be downloaded
		Index: 0,
		Length: 1000,
	}

	data := make([]byte, 1000) // creates a fake piece of data, this represents what the peer sends back
	payload := make([]byte, 8+len(data)) // the 8 bytes extra are 4 bytes for index and 4 bytes for begin

	copy(payload[8:], data) // this places the data after the first 8 bytes into the data section


	msg := peer.Message{
		// This simulates the message sent by a peer
		ID:      peer.MsgPiece,
		Payload: payload,
	}


	msg.Serialize() // this converts the message into raw bytes for network transmission

	conn := &mockConn{
		// this simulates a peer connection
		readBuffer: msg.Serialize(),
	}
	
	result, err := downloadPiece(conn, work) // the process then runs
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1000 {
		t.Fatalf("expected 1000 bytes got %d", len(result)) // verified the function returned the correct piece length
	}
}

func TestDownloadPiece_MultipleBlocks(t *testing.T) {
	work := PieceWork{
		Index: 0,
		Length: BlockSize * 2,
	}

	data := make([]byte, work.Length)

	// first block
	payload1 := make([]byte, 8+BlockSize)
	binary.BigEndian.PutUint32(payload1[0:4], 0)
	binary.BigEndian.PutUint32(payload1[4:8], 0)
	copy(payload1[8:], data[:BlockSize])

	msg1 := peer.Message{
		ID:      peer.MsgPiece,
		Payload: payload1,
	}

	// second block
	payload2 := make([]byte, 8+BlockSize)
	binary.BigEndian.PutUint32(payload2[0:4], 0)
	binary.BigEndian.PutUint32(payload2[4:8], BlockSize)
	copy(payload2[8:], data[BlockSize:])

	msg2 := peer.Message{
		ID:      peer.MsgPiece,
		Payload: payload2,
	}

	conn := &mockConn{
		readBuffer: append(msg1.Serialize(), msg2.Serialize()...),
	}

	result, err  := downloadPiece(conn, work)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != work.Length {
		t.Fatalf("expected %d for %d", work.Length, len(result))
	}
}

func TestDownloadPiece_WrongMessage(t *testing.T) {
	work := PieceWork{
		Index: 0,
		Length: 1000,
	}

	msg := peer.Message{
		ID: peer.MsgChoke,
	}

	conn := &mockConn{
		readBuffer: msg.Serialize(),
	}

	_, err := downloadPiece(conn, work)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

func TestDownloadPiece_WriteError(t *testing.T) {
	work := PieceWork{
		Index: 0,
		Length : 1000,
	}

	conn := &failConn{}

	_, err := downloadPiece(conn, work)

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadFromPeer_HashMatch(t *testing.T) {
	data := make([]byte, 1000)

	work := PieceWork{
		Index: 0,
		Length: 1000,
		Hash: sha1.Sum(data),
	}

	pw := make(chan PieceWork, 1)
	pR := make(chan PieceResult, 1)

	pw <- work
	close(pw)

	 // Build what the "peer" sends: bitfield + unchoke + piece data
    bf := peer.Message{ID: peer.MsgBitfield, Payload: []byte{0xff}}
    unchoke := peer.Message{ID: peer.MsgUnchoke}
    piece := buildPieceMessage(0, 0, data)

    var buf []byte
    buf = append(buf, bf.Serialize()...)
    buf = append(buf, unchoke.Serialize()...)
    buf = append(buf, piece...)

    conn := &mockConn{readBuffer: buf}

    err := runPeerSession(conn, fakeTorrent, pw, pR)
    // err is expected (channel closed), check result
	if err != nil {
    	t.Fatalf("runPeerSession returned error: %v", err)
	}

    select {
    case result := <-pR:
        if len(result.Data) != 1000 {
            t.Fatalf("wrong data length: %d", len(result.Data))
        }
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for piece result")
	}
}


// Peer sends unchoke as first message (no bitfield) — tests alreadyUnchoked path
func TestRunPeerSession_UnchokeFirst(t *testing.T) {
    data := make([]byte, 1000)
    pw := make(chan PieceWork, 1)
    pR := make(chan PieceResult, 1)
    pw <- PieceWork{Index: 0, Length: 1000, Hash: sha1.Sum(data)}
    close(pw)

    unchoke := peer.Message{ID: peer.MsgUnchoke}
    piece := buildPieceMessage(0, 0, data)

    var buf []byte
    buf = append(buf, unchoke.Serialize()...)
    buf = append(buf, piece...)

    conn := &mockConn{readBuffer: buf}
    err := runPeerSession(conn, fakeTorrent, pw, pR)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    select {
    case result := <-pR:
        if len(result.Data) != 1000 {
            t.Fatalf("wrong data length: %d", len(result.Data))
        }
    case <-time.After(time.Second):
        t.Fatal("timed out")
    }
}

// Peer sends an unexpected first message — should return error
func TestRunPeerSession_UnexpectedFirstMessage(t *testing.T) {
    pw := make(chan PieceWork, 1)
    pR := make(chan PieceResult, 1)
    close(pw)

    msg := peer.Message{ID: peer.MsgChoke}
    conn := &mockConn{readBuffer: msg.Serialize()}

    err := runPeerSession(conn, fakeTorrent, pw, pR)
    if err == nil {
        t.Fatal("expected error for unexpected first message")
    }
}

// Hash mismatch — piece should NOT appear in results
func TestRunPeerSession_HashMismatch(t *testing.T) {
    data := make([]byte, 1000)
    badHash := [20]byte{0xff} // wrong hash

    pw := make(chan PieceWork, 2) // buffer=2 so requeue doesn't block
    pR := make(chan PieceResult, 1)
    pw <- PieceWork{Index: 0, Length: 1000, Hash: badHash}
    close(pw)

    bf := peer.Message{ID: peer.MsgBitfield, Payload: []byte{0xff}}
    unchoke := peer.Message{ID: peer.MsgUnchoke}
    piece := buildPieceMessage(0, 0, data)

    var buf []byte
    buf = append(buf, bf.Serialize()...)
    buf = append(buf, unchoke.Serialize()...)
    buf = append(buf, piece...)

    conn := &mockConn{readBuffer: buf}
    runPeerSession(conn, fakeTorrent, pw, pR)

    select {
    case <-pR:
        t.Fatal("should not have received result with bad hash")
    default:
        // correct — nothing sent
    }
}

// Peer doesn't have the piece (bitfield says 0) — work requeued, nothing downloaded
func TestRunPeerSession_PeerMissingPiece(t *testing.T) {
    pw := make(chan PieceWork, 2)
    pR := make(chan PieceResult, 1)
    pw <- PieceWork{Index: 0, Length: 1000, Hash: sha1.Sum(make([]byte, 1000))}
    close(pw)

    bf := peer.Message{ID: peer.MsgBitfield, Payload: []byte{0x00}} // no pieces
    unchoke := peer.Message{ID: peer.MsgUnchoke}

    var buf []byte
    buf = append(buf, bf.Serialize()...)
    buf = append(buf, unchoke.Serialize()...)

    conn := &mockConn{readBuffer: buf}
    runPeerSession(conn, fakeTorrent, pw, pR)

    select {
    case <-pR:
        t.Fatal("should not have received result when peer lacks piece")
    default:
    }
}

// Wrong piece index in response
func TestDownloadPiece_WrongIndex(t *testing.T) {
    work := PieceWork{Index: 0, Length: 1000}

    payload := make([]byte, 8+1000)
    binary.BigEndian.PutUint32(payload[0:4], 99) // wrong index
    binary.BigEndian.PutUint32(payload[4:8], 0)

    msg := peer.Message{ID: peer.MsgPiece, Payload: payload}
    conn := &mockConn{readBuffer: msg.Serialize()}

    _, err := downloadPiece(conn, work)
    if err == nil {
        t.Fatal("expected error for wrong piece index")
    }
}

// Wrong begin offset in response
func TestDownloadPiece_WrongBeginOffset(t *testing.T) {
    work := PieceWork{Index: 0, Length: 1000}

    payload := make([]byte, 8+1000)
    binary.BigEndian.PutUint32(payload[0:4], 0)
    binary.BigEndian.PutUint32(payload[4:8], 500) // wrong begin

    msg := peer.Message{ID: peer.MsgPiece, Payload: payload}
    conn := &mockConn{readBuffer: msg.Serialize()}

    _, err := downloadPiece(conn, work)
    if err == nil {
        t.Fatal("expected error for wrong begin offset")
    }
}

// Payload too short (less than 8 bytes)
func TestDownloadPiece_ShortPayload(t *testing.T) {
    work := PieceWork{Index: 0, Length: 1000}

    msg := peer.Message{ID: peer.MsgPiece, Payload: []byte{0x00, 0x01}}
    conn := &mockConn{readBuffer: msg.Serialize()}

    _, err := downloadPiece(conn, work)
    if err == nil {
        t.Fatal("expected error for short payload")
    }
}

// Block data exceeds piece size
func TestDownloadPiece_BlockExceedsPieceSize(t *testing.T) {
    work := PieceWork{Index: 0, Length: 100}

    oversizedData := make([]byte, 200) // larger than piece
    payload := make([]byte, 8+len(oversizedData))
    binary.BigEndian.PutUint32(payload[0:4], 0)
    binary.BigEndian.PutUint32(payload[4:8], 0)
    copy(payload[8:], oversizedData)

    msg := peer.Message{ID: peer.MsgPiece, Payload: payload}
    conn := &mockConn{readBuffer: msg.Serialize()}

    _, err := downloadPiece(conn, work)
    if err == nil {
        t.Fatal("expected error when block exceeds piece size")
    }
}
