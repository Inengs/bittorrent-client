package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"

	"github.com/Inengs/bittorrent-client/internal/bencode"
	"github.com/Inengs/bittorrent-client/internal/download"
	"github.com/Inengs/bittorrent-client/internal/torrent"
	"github.com/Inengs/bittorrent-client/internal/tracker"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: bittorrent-client <file.torrent> <output file>")
	}

	// get the torrent file name from the command
	path := os.Args[1]

	// open the torrent file
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err) // prints and returns
	}
	defer file.Close()
	
	// parse it
	bencodeTorrent, err := bencode.Open(file)
	if err != nil {
		log.Fatal(err)
	}


	// convert to TorrentFile
	torrentFile, err := torrent.ToTorrentFile(*bencodeTorrent)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("tracker:", torrentFile.Announce)
	
	// generate peerID
	var peerID [20]byte
	_, err = rand.Read(peerID[:])
	if err != nil {
		log.Fatal(err)
	}

	// get peers
	peers, err := tracker.GetPeers(torrentFile, peerID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("found %d peers\n", len(peers))

	fmt.Printf("found %d peers\n", len(peers))
	for _, p := range peers {
    	fmt.Printf("  %s:%d\n", p.IP, p.Port)
	}

	// download
	data, err := download.Download(torrentFile, peers, peerID)
	if err != nil {
		log.Fatal(err)
	}

	// write to output File
	err = os.WriteFile(os.Args[2], data, 0644)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(peers)
	fmt.Println("download complete!")
}