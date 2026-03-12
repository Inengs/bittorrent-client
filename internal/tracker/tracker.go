package tracker

import (
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Inengs/bittorrent-client/internal/peer"
	"github.com/Inengs/bittorrent-client/internal/torrent"
	bencodelib "github.com/jackpal/bencode-go"
)

type bencodeTrackerResponse struct {
	Interval int `bencode:"interval"`
	// Peers is a compact string — every 6 bytes is one peer (4 bytes IP + 2 bytes port).
	Peers string `bencode:"peers"`
}

// http://tracker.example.com/announce?info_hash=...&peer_id=...&port=6881&uploaded=0&downloaded=0&compact=1&left=...
func buildTrackerURL(torrentFile torrent.TorrentFile, peerID [20]byte) (string, error){
	base, err := url.Parse(torrentFile.Announce)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("info_hash", string(torrentFile.InfoHash[:]))
	params.Set("peer_id", string(peerID[:]))
	params.Set("port", "6881")
	params.Set("uploaded", "0")
	params.Set("downloaded", "0")
	params.Set("compact", "1")
	params.Set("left", strconv.Itoa(torrentFile.Length))
	params.Set("numwant", "50") // ask for up to 50 peers

	base.RawQuery = params.Encode()

	return base.String(), nil
}

func GetPeers(torrentFile torrent.TorrentFile, peerID [20]byte) ([]peer.Peer, error) {
	url, err := buildTrackerURL(torrentFile, peerID)
	if err != nil {
		return []peer.Peer{}, err
	}

	response, err := http.Get(url) // send your get request and get the response gives a *http.Response, it is the Body part of it that is an io.Reader
	if err != nil {
		panic (err)
	}
	defer response.Body.Close() // close the response body to avoid memory leaks

	trackerResponse := new(bencodeTrackerResponse)
	err = bencodelib.Unmarshal(response.Body, trackerResponse)
	if err != nil {
		return []peer.Peer{}, err
	}

	peers, err := splitPeers(trackerResponse.Peers)
	if err != nil {
		return []peer.Peer{}, err
	}

	return peers, nil
}

func splitPeers(Peers string) ([]peer.Peer, error){
	peerBytes := []byte(Peers)
	if len(peerBytes) % 6 != 0{
		return []peer.Peer{}, errors.New("malformed peers string")
	}

	numPeers := len(peerBytes) / 6
    result := make([]peer.Peer, numPeers)

	for i := 0; i < numPeers; i++ {
        result[i] = peer.Peer{
            IP:   net.IP(peerBytes[i*6 : i*6+4]),
            Port: binary.BigEndian.Uint16(peerBytes[i*6+4 : i*6+6]),
        }
	}

	return result, nil
}