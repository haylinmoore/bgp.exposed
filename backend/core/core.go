package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/websocket/v2"
	"github.com/hamptonmoore/bgp.exposed/backend/bgp"
	"github.com/hamptonmoore/bgp.exposed/backend/common"
)

var addr = flag.String("addr", "localhost:8080", "http service address")

var server *bgp.BGPServer

func ClientHandler(c *websocket.Conn) {
	var peer *bgp.Peer

	started := make(chan bool, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started

		go func() {
			for {
				select {
				case val := <-peer.SendChan:
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
			cancel()
			break
		}
		var packet common.Packet
		err = json.Unmarshal(message, &packet)
		if err != nil {
			log.Println(err)
			continue
		}

		data, _ := json.Marshal(packet.Data)
		if peer == nil && packet.Type == "CreateRequest" {
			v := common.CreateRequest{}
			json.Unmarshal(data, &v)
			peer = server.CreatePeer(&v, ctx, cancel)
			go peer.Handler(started)
		}
		if peer != nil {
			if packet.Type == "RouteData" {
				v := common.RouteData{}
				json.Unmarshal(data, &v)
				peer.RoutesToAnnounce <- &v
			}
		}

	}
}

func main() {
	app := fiber.New()
	server = bgp.CreateBGPServer(1000, "0.0.0.0:2000", "1.1.1.1")
	flag.Parse()
	log.SetFlags(0)

	app.Use(cors.New())

	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/", websocket.New(ClientHandler))

	log.Fatal(app.Listen(*addr))
}
