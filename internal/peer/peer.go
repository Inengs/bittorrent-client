package peer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"time"
)

type Peer struct {
	IP net.IP
	Port uint16
}

type Handshake struct {
	ProtocolStringLength [1]byte
	ProtocolString [19]byte
	ReservedBytes [8]byte
	InfoHash [20]byte
	PeerID [20]byte
}

type Message struct {
	ID uint8
	Payload []byte
}

const (
    MsgChoke     uint8 = 0
    MsgUnchoke   uint8 = 1
    MsgInterested uint8 = 2
    MsgHave      uint8 = 4
    MsgBitfield  uint8 = 5
    MsgRequest   uint8 = 6
    MsgPiece     uint8 = 7
)

func (m *Message) Serialize() []byte {
	var buffer bytes.Buffer

	length := 1+ len(m.Payload)
	binary.Write(&buffer, binary.BigEndian, uint32(length))
	buffer.WriteByte(m.ID)
	buffer.Write(m.Payload)

	return buffer.Bytes()
}

func (h *Handshake) Serialize() []byte {
	var buffer bytes.Buffer
	b := make([]byte, 8)

	buffer.WriteByte(19)
	buffer.WriteString("BitTorrent protocol")
	buffer.Write(b)
	buffer.Write(h.InfoHash[:])
	buffer.Write(h.PeerID[:])

	return buffer.Bytes()
}

func ReadMessage(r io.Reader) (*Message, error) {
	// read message length
	lengthBuffer := make([]byte, 4)
	_, err := io.ReadFull(r, lengthBuffer)
	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBuffer)

	// read message ID
	messageIDBuffer := make([]byte, 1)
	_, err = io.ReadFull(r, messageIDBuffer)
	if err != nil {
		return nil, err
	}

	// read payload
	payloadBuffer := make([]byte, length -1) 
	_, err = io.ReadFull(r, payloadBuffer)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID: messageIDBuffer[0],
		Payload: payloadBuffer,
	}, nil
}

func Read(r io.Reader) (*Handshake, error) {
	// read protocol length (1 byte)
	lengthBuffer := make([]byte, 1)
	_, err := io.ReadFull(r, lengthBuffer)
	if err != nil {
		return nil, err
	}
	protocolLength := int(lengthBuffer[0])

	// read protocol string
	protocolBuffer := make([]byte, protocolLength)
	_, err = io.ReadFull(r, protocolBuffer)
	if err != nil {
		return nil, err
	}

	//read reserved bytes
	reservedBuffer := make([]byte, 8)
	_, err = io.ReadFull(r, reservedBuffer)
	if err != nil {
		return nil, err
	}

	// read infoHash 
	var infoHash [20]byte
	_, err = io.ReadFull(r, infoHash[:])
	if err != nil {
		return nil, err
	}

	// read PeerID
	var peerID [20]byte
	_, err = io.ReadFull(r, peerID[:])	
	if err != nil {
		return nil, err
	}

	return &Handshake{
		InfoHash: infoHash,
		PeerID: peerID,
	}, nil
}

func Connect(peer Peer, infoHash [20]byte, peerID [20]byte) (net.Conn, error) {
	addr := net.JoinHostPort(peer.IP.String(), strconv.Itoa(int(peer.Port)))
	conn, err := net.DialTimeout("tcp", addr, 3 * time.Second)
	if err != nil {
		return nil, err
	}

	newHandshake := new(Handshake)
	newHandshake.InfoHash = infoHash
	newHandshake.PeerID = peerID

	_, err = conn.Write(newHandshake.Serialize())
	if err != nil {
		return nil, err
	}

	response, err := Read(conn)
	if err != nil {
		return nil, err
	}

	if response.InfoHash != infoHash {
		conn.Close()
		return nil, errors.New("infohash mismatch")
	}

	return conn, nil
}