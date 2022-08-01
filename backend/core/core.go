package main

import (
	"fmt"

	"github.com/hamptonmoore/bgp.exposed/backend/bgp"
	"github.com/hamptonmoore/bgp.exposed/backend/common"
)

func main() {

	server := bgp.CreateBGPServer(1000, "0.0.0.0:2000", "1.1.1.1")

	rc := server.CreatePeer(&common.CreateRequest{
		LocalASN: 64512,
		PeerASN:  923,
		PeerIP:   "198.51.100.1",
	})

	// GET UPDATES FOR A PEER
	for {
		fmt.Printf("Got update: %v\n", <-rc)
	}
}
