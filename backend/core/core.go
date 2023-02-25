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
	"github.com/sirupsen/logrus"
)

var (
	httpAddr      = flag.String("http.addr", "0.0.0.0", "HTTP listen address")
	httpPort      = flag.Int("http.port", 8080, "HTTP listen port")
	bgpAddr       = flag.String("bgp.addr", "0.0.0.0", "BGP listen address")
	bgpPublicAddr = flag.String("bgp.publicAddr", "", "BGP public listen address. Defaults to bgp.addr but cannot be 0.0.0.0.")
	bgpPort       = flag.Int("bgp.port", 2000, "BGP listen port")
	bgpRouterId   = flag.String("bgp.routerId", "", "BGP router ID. Defaults to bgp.publicAddr.")
	logLevel      = flag.String("log.level", "info", "Log level can be trace, debug, info, warn, or error")
	logTimestamp  = flag.Bool("log.timestamp", true, "Show timestamp in logs. Disable if you are using an external logging system like systemd.")
)

var server *bgp.BGPServer
var log *logrus.Logger

//go:embed routesets.json
var routesets []byte

func ClientHandler(c *websocket.Conn) {
	log.Debugf("[ClientHandler %p] started for client %s", &c, c.RemoteAddr().String())
	var peer *bgp.Peer

	started := make(chan bool, 1)
	ctx, cancel := context.WithCancel(context.Background())

	// send initial data to client
	data, _ := json.Marshal(common.Packet{
		Type: "InitData",
		Data: common.InitData{
			RouterId: *bgpRouterId,
			ListenIp: *bgpPublicAddr,
		},
	})
	c.WriteMessage(1, data)

	// Start a goroutine to handle messages from the bgp server
	go func() {
		log.Tracef("[ClientHandler %p][peer->web] waiting for bgp server peer create", &c)
		// Don't do anything until we get a message on the "started" channel
		<-started

		go func() {
			log.Tracef("[ClientHandler %p][peer->web] received \"started\" message, starting loop", &c)
			for {
				select {
				case val := <-peer.SendChan:
					log.Tracef("[ClientHandler %p][peer->web] received message from peer, passing to client: %+v", &c, val)
					// take received data, convert to JSON, and send over the WS to the client
					data, _ := json.Marshal(val)
					c.WriteMessage(1, data)
				case <-ctx.Done():
					log.Debugf("[ClientHandler %p][peer->web] websocket closed, ending goroutine", &c)
					// If WS closes, return
					return
				}
			}
		}()
	}()

	log.Tracef("[ClientHandler %p] starting main loop for processing web->peer websocket messages", &c)
	for {
		// Receive a message over the WS
		_, message, err := c.ReadMessage()
		log.Tracef("[ClientHandler %p] received message: %s", &c, message)
		if err != nil {
			log.Warnf("[ClientHandler %p] error reading message, discarding: %s", &c, err)
			break
		}

		// Unpack it into the Packet struct
		var packet common.Packet
		if err := json.Unmarshal(message, &packet); err != nil {
			log.Warnf("[ClientHandler %p] error unmarshalling packet, discarding: %s", &c, err)
			continue
		}

		// Grab just the "data" field in JSON form
		data, err := json.Marshal(packet.Data)
		if err != nil {
			log.Warnf("[ClientHandler %p] error marshalling packet data, discarding: %s", &c, err)
			continue
		}
		// If we haven't already created a BGP server
		if peer == nil {
			// If we get a CreateRequest packet
			if packet.Type == "CreateRequest" {
				log.Tracef("[ClientHandler %p] packet is CreateRequest", &c)
				// Unpack packet's "data" field into a struct
				v := common.CreateRequest{}
				if err := json.Unmarshal(data, &v); err != nil {
					log.Warnf("[ClientHandler %p] error unmarshalling CreateRequest, discarding: %s", &c, err)
					break
				}
				log.Infof("[ClientHandler %p] %s requested to create peer on bgp server: %+v", &c, c.RemoteAddr().String(), v)
				
				// Create the BGP server using the data we extracted
				peer, err = server.CreatePeer(&v, ctx, cancel)
				if err != nil {
					log.Warnf("[ClientHandler %p] peer create failed: %s", &c, err)
					data, _ := json.Marshal(common.Packet{
						Type: "Error",
						Data: common.Error{
							Message: err.Error(),
						},
					})
					c.WriteMessage(1, data)
				} else {
					log.Tracef("[ClientHandler %p] peer create succeeded, sending message to peer->web goroutine and starting peer handler", &c)
					started <- true
					go peer.Handler()
				}
			} else {
				log.Warnf("[ClientHandler %p] Got invalid request type for current state, discarding", &c)
				data, _ := json.Marshal(common.Packet{
					Type: "Error",
					Data: common.Error{
						Message: "Invalid request type for current state",
					},
				})
				c.WriteMessage(1, data)
			}
		// If we've already created a BGP server
		} else if peer != nil {
			// then a CreateRequest is not valid
			if packet.Type == "CreateRequest" {
				log.Warnf("[ClientHandler %p] Got CreateRequest but already created peer", &c)
				data, _ := json.Marshal(common.Packet{
					Type: "Error",
					Data: common.Error{
						Message: "Invalid request type for current state",
					},
				})
				c.WriteMessage(1, data)
			} else if packet.Type == "RouteData" {
				log.Tracef("[ClientHandler %p] packet is RouteData", &c)
				// Unpack packet's "data" field into a struct
				v := common.RouteData{}
				if err := json.Unmarshal(data, &v); err != nil {
					log.Warnf("[ClientHandler %p] error unmarshalling RouteData, discarding: %s", &c, err)
					break
				}
				log.Infof("[ClientHandler %p] announcing/withdrawing routes: %+v", &c, v)
				// Send struct to BGP server
				peer.RoutesToAnnounce <- &v
			} else {
				log.Warnf("[ClientHandler %p] unknown or invalid packet type, discarding: %s", &c, packet.Type)
			}
		}
	}
	if peer != nil {
		peer.KeepAlive <- &messages.BGPMessageKeepAlive{}
	}
	cancel()
	time.Sleep(time.Second * 5)
	log.Debugf("[ClientHandler %p] ending", &c)
}

func main() {
	flag.Parse()

	// disable logging in underlying libraries
	logrus.SetLevel(logrus.PanicLevel)
	 
	log = logrus.New()

	logFormat := &logrus.TextFormatter{
		FullTimestamp: *logTimestamp,
	}
	log.SetFormatter(logFormat)

	switch *logLevel {
	case "trace":
		log.SetLevel(logrus.TraceLevel)
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		// default
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.Fatalf("Invalid log level \"%s\"", *logLevel)
	}
	log.Infof("[main] Log level set to %s", *logLevel)

	// Remove whitespace
	routesets = []byte(regexp.MustCompile(`\s+`).ReplaceAllString(string(routesets), ""))

	// if bgp.addr is 0.0.0.0, bgp.publicAddr must be set, and we log it for clarity
	if *bgpAddr == "0.0.0.0" {
		if *bgpPublicAddr != "" {
			// bgp.routerId defaults to bgp.publicAddr
			if *bgpRouterId == "" {
				*bgpRouterId = *bgpPublicAddr
			}
			log.Infof("[main] Starting BGP server on %s:%d (public IP %s) with router ID %s", *bgpAddr, *bgpPort, *bgpPublicAddr, *bgpRouterId)
		} else {
			log.Fatalf("Must specify non-0.0.0.0 bgp.publicAddr")
		}
		
	} else {
		*bgpPublicAddr = *bgpAddr

		// bgp.routerId defaults to bgp.publicAddr
		if *bgpRouterId == "" {
			*bgpRouterId = *bgpPublicAddr
		}
		log.Infof("[main] Starting BGP server on %s:%d with router ID %s", *bgpAddr, *bgpPort, *bgpRouterId)
	}

	server = bgp.CreateBGPServer(1000, fmt.Sprintf("%s:%d",*bgpAddr, *bgpPort), *bgpRouterId, log)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(cors.New())

	// Use middleware to upgrade "/ws" requests to a WebSocket
	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Serve requests to /ws via ClientHandler
	app.Get("/ws/", websocket.New(ClientHandler))

	// Serve static "routesets.json" file
	app.Get("/routesets.json", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(routesets)
	})

	log.Infof("[main] Starting HTTP API on %s:%d", *httpAddr, *httpPort)
	log.Fatal(app.Listen(fmt.Sprintf("%s:%d",*httpAddr, *httpPort)))
}
