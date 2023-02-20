package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"regexp"
	"time"

	_ "embed"

	"github.com/bgptools/fgbgp/messages"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/websocket/v2"
	"github.com/hamptonmoore/bgp.exposed/backend/bgp"
	"github.com/hamptonmoore/bgp.exposed/backend/common"
	log "github.com/sirupsen/logrus"
)

var (
	httpAddr    = flag.String("http.addr", "0.0.0.0", "http listen address")
	httpPort    = flag.Int("http.port", 8080, "http listen port")
	bgpAddr     = flag.String("bgp.addr", "0.0.0.0", "bgp listen address")
	bgpPort     = flag.Int("bgp.port", 2000, "bgp listen port")
	bgpRouterId = flag.String("bgp.routerId", "1.1.1.1", "bgp router ID")
	logLevel     = flag.String("log.level", "info", "log level can be trace, debug, info, warn, error, fatal, or panic")
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
				break
			}
			peer, err = server.CreatePeer(&v, ctx, cancel)
			if err != nil {
				data, _ := json.Marshal(common.Packet{
					Type: "Error",
					Data: common.Error{
						Message: err.Error(),
					},
				})
				c.WriteMessage(1, data)
			} else {
				started <- true
				go peer.Handler()
			}
		} else if peer != nil {
			if packet.Type == "RouteData" {
				v := common.RouteData{}
				if err := json.Unmarshal(data, &v); err != nil {
					log.Warnf("RouteData unmarshal: %s", err)
					break
				}
				peer.RoutesToAnnounce <- &v
			}
		} else {
			log.Debugf("Unknown packet type: %s", packet.Type)
		}
	}
	if peer != nil {
		peer.KeepAlive <- &messages.BGPMessageKeepAlive{}
	}
	cancel()
	time.Sleep(time.Second * 5)
}

func main() {
	flag.Parse()

	switch *logLevel {
	case "trace":
		log.SetLevel(log.TraceLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		// default
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "panic":
		log.SetLevel(log.PanicLevel)
	default:
		log.Fatalf("Invalid log level \"%s\"", *logLevel)
	}
	log.Infof("Log level set to %s", *logLevel)

	// Remove whitespace
	routesets = []byte(regexp.MustCompile(`\s+`).ReplaceAllString(string(routesets), ""))

	log.Infof("Starting BGP server on %s:%d with router ID %s", *bgpAddr, *bgpPort, *bgpRouterId)
	server = bgp.CreateBGPServer(1000, fmt.Sprintf("%s:%d",*bgpAddr, *bgpPort), *bgpRouterId)

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

	log.Infof("Starting HTTP API on %s:%d", *httpAddr, *httpPort)
	log.Fatal(app.Listen(fmt.Sprintf("%s:%d",*httpAddr, *httpPort)))
}
