package main

import (
	"log"

	"github.com/hamptonmoore/bgp.exposed/backend/bgp"
	"github.com/hamptonmoore/bgp.exposed/backend/common"
)

func main() {

	server := bgp.CreateBGPServer(1000, "0.0.0.0:2000", "1.1.1.1")

	peer := server.CreatePeer(&common.CreateRequest{
		LocalASN: 64512,
		PeerASN:  923,
		PeerIP:   "198.51.100.1",
	})

	peer.RoutesToAnnounce <- &common.RouteData{
		Prefixes:    []string{"1.1.1.1/32", "9.9.9.0/23"},
		AsPath:      []uint32{179, 13335},
		Communities: [][]uint16{{1, 2}, {179, 2473}},
		NextHop:     "8.8.8.8",
	}

	go peer.Handler()

	for {
		log.Println(<-peer.RouteChannel)
	}
}
