package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/hamptonmoore/bgp.exposed/backend/bgp"
	"github.com/hamptonmoore/bgp.exposed/backend/common"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var upgrader = websocket.Upgrader{} // use default options

var server *bgp.BGPServer

func ClientHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	var peer *bgp.Peer

	started := make(chan bool, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started

		go func() {
			for {
				select {
				case val := <-peer.RouteChannel:
					data, _ := json.Marshal(val)
					c.WriteMessage(1, data)
				case <-ctx.Done():
					return
				}
			}
		}()
	}()

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		var packet common.Packet
		err = json.Unmarshal(message, &packet)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println(packet.Type)
		data, _ := json.Marshal(packet.Data)
		if peer == nil && packet.Type == "CreateRequest" {
			v := common.CreateRequest{}
			json.Unmarshal(data, &v)
			peer = server.CreatePeer(&v, ctx, cancel)
			go peer.Handler(started)
		} else {
			cancel()
		}

	}
}

func main() {
	server = bgp.CreateBGPServer(1000, "0.0.0.0:2000", "1.1.1.1")

	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/ws", ClientHandler)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
