package main

import (
	"context"
	"encoding/json"
	"flag"
	"regexp"

	_ "embed"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/websocket/v2"
	"github.com/hamptonmoore/bgp.exposed/backend/bgp"
	"github.com/hamptonmoore/bgp.exposed/backend/common"
	log "github.com/sirupsen/logrus"
)

var (
	addr    = flag.String("addr", "localhost:8080", "http service address")
	verbose = flag.Bool("v", false, "enable verbose logging")
)

var server *bgp.BGPServer

//go:embed routesets.json
var routesets []byte

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
		log.Debugf("Received message: %s", message)
		if err != nil {
			log.Warnf("read: %s", err)
			cancel()
			break
		}

		var packet common.Packet
		if err := json.Unmarshal(message, &packet); err != nil {
			log.Warnf("unmarshal: %s", err)
			continue
		}

		data, err := json.Marshal(packet.Data)
		if err != nil {
			log.Warnf("marshal: %s", err)
			continue
		}
		if peer == nil && packet.Type == "CreateRequest" {
			v := common.CreateRequest{}
			if err := json.Unmarshal(data, &v); err != nil {
				log.Warnf("CreateRequest unmarshal: %s", err)
				cancel()
				break
			}
			peer = server.CreatePeer(&v, ctx, cancel)
			go peer.Handler(started)
		} else if peer != nil {
			if packet.Type == "RouteData" {
				v := common.RouteData{}
				if err := json.Unmarshal(data, &v); err != nil {
					log.Warnf("RouteData unmarshal: %s", err)
					cancel()
					break
				}
				peer.RoutesToAnnounce <- &v
			}
		} else {
			log.Debugf("Unknown packet type: %s", packet.Type)
		}
	}
}

func main() {
	flag.Parse()
	if *verbose {
		log.SetLevel(log.DebugLevel)
		log.Debug("Verbose logging enabled")
	}

	// Remove whitespace
	routesets = []byte(regexp.MustCompile(`\s+`).ReplaceAllString(string(routesets), ""))

	server = bgp.CreateBGPServer(1000, "0.0.0.0:2000", "1.1.1.1")

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
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
	app.Get("/routesets.json", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(routesets)
	})

	log.Infof("Starting API on %s", *addr)
	log.Fatal(app.Listen(*addr))
}
