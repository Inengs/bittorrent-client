package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"

	"github.com/Inengs/bittorrent-client/internal/bencode"
	"github.com/Inengs/bittorrent-client/internal/torrent"
	"github.com/Inengs/bittorrent-client/internal/tracker"
)

func main() {
	path := os.Args[1]

	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err) // prints and returns
	}
	defer file.Close()
	
	bencodeTorrent, err := bencode.Open(file)
	if err != nil {
		log.Fatal(err)
	}

	torrentFile, err := torrent.ToTorrentFile(*bencodeTorrent)
	if err != nil {
		log.Fatal(err)
	}

	var peerID [20]byte
	_, err = rand.Read(peerID[:])
	if err != nil {
		log.Fatal(err)
	}

	peers, err := tracker.GetPeers(torrentFile, peerID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(peers)
}