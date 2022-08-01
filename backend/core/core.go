package main

import (
	"time"

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
		Prefix:  "1.1.1.1/32",
		AsPath:  []uint32{179, 13335},
		NextHop: "8.8.8.8",
	}

	go peer.Handler()

	for {
		time.Sleep(time.Second * 10)
	}
}
