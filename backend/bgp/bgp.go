package bgp

import (
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/bgptools/fgbgp/messages"
	fgbgp "github.com/bgptools/fgbgp/server"
	"github.com/hamptonmoore/bgp.exposed/backend/common"
)

type Peer struct {
	Key              string
	PeerASN          uint32 `json:"peerASN"`
	PeerIP           string `json:"peerIP"`
	LocalASN         uint32 `json:"localASN"`
	Server           *BGPServer
	KeepAlive        chan *messages.BGPMessageKeepAlive
	Neighbor         *fgbgp.Neighbor
	RouteChannel     chan *common.RouteData
	RoutesToAnnounce chan *common.RouteData
	EOL              chan bool
}

func (p *Peer) Handler(started chan bool) {
	// Wait for the peer to raise
	<-p.KeepAlive
	started <- true
main:
	for {
		select {
		case <-time.After(time.Second * 10):
			if p.Neighbor != nil {
				p.Neighbor.OutQueue <- messages.BGPMessageKeepAlive{}
			}
		case route := <-p.RoutesToAnnounce:
			pa := []messages.BGPAttributeIf{
				messages.BGPAttribute_ORIGIN{
					Origin: byte(route.Origin),
				},
				messages.BGPAttribute_NEXTHOP{NextHop: net.ParseIP(route.NextHop)},
				messages.BGPAttribute_ASPATH{Segments: []messages.ASPath_Segment{
					{
						SType:  2,
						ASPath: route.AsPath,
					},
				}},
			}
			var communities []uint32
			for _, c := range route.Communities {
				communities = append(communities, uint32(c[1])+(uint32(c[0])*65536))
			}
			pa = append(pa, messages.BGPAttribute_COMMUNITIES{
				Communities: communities,
			})

			announcement := &messages.BGPMessageUpdate{
				PathAttributes: pa,
			}

			for i, prefix := range route.Prefixes {
				_, pref, _ := net.ParseCIDR(prefix)

				announcement.NLRI = append(announcement.NLRI, messages.NLRI_IPPrefix{
					Prefix: *pref,
					PathId: uint32(time.Now().UTC().Unix()) + uint32(i),
				})
			}

			p.Neighbor.OutQueue <- announcement
		case <-p.EOL:
			//log.Println(p.Neighbor.State.CurState)
			p.Neighbor.Disconnect()
			p.Server.PeerLock.Lock()
			delete(p.Server.Peers, p.Key)
			p.Server.PeerLock.Unlock()
			break main
		}
	}
	log.Println("Worker has died")
}

type BGPServer struct {
	Fgbgp    *fgbgp.Manager
	PeerLock sync.RWMutex
	Peers    map[string]*Peer
}

func (s *BGPServer) GetPeerFromNeigh(n *fgbgp.Neighbor) (*Peer, bool) {
	s.PeerLock.Lock()
	key := n.Addr.String() + "|" + strconv.FormatUint(uint64(n.PeerASN), 10)
	defer s.PeerLock.Unlock()

	peer, ok := s.Peers[key]
	return peer, ok
}

func (s *BGPServer) CreatePeer(request *common.CreateRequest) *Peer {
	s.PeerLock.Lock()

	peer := &Peer{
		Key:              request.ToKey(),
		PeerASN:          request.PeerASN,
		LocalASN:         request.LocalASN,
		PeerIP:           request.PeerIP,
		Server:           s,
		RouteChannel:     make(chan *common.RouteData, 1024),
		KeepAlive:        make(chan *messages.BGPMessageKeepAlive, 1),
		RoutesToAnnounce: make(chan *common.RouteData, 1024),
		EOL:              make(chan bool, 16),
	}
	s.Peers[request.ToKey()] = peer
	s.PeerLock.Unlock()
	return peer
}

func (s *BGPServer) Notification(msg *messages.BGPMessageNotification, n *fgbgp.Neighbor) bool {
	log.Printf("Notification: %v", msg)
	return true
}

func (s *BGPServer) ProcessReceived(msg interface{}, n *fgbgp.Neighbor) (bool, error) {
	// log.Printf("ProcessReceived: %s", msg)
	n.LocalLastKeepAliveRecv = time.Now()
	switch v := msg.(type) {
	case *messages.BGPMessageOpen:
		s.PeerLock.Lock()
		defer s.PeerLock.Unlock()
		key := n.Addr.String() + "|" + strconv.FormatUint(uint64(v.ASN), 10)
		if peer, ok := s.Peers[key]; ok {
			n.ASN = peer.LocalASN
			peer.Neighbor = n
			return true, nil
		} else {
			return false, nil
		}
	case *messages.BGPMessageKeepAlive:
		peer, ok := s.GetPeerFromNeigh(n)
		n.OutQueue <- messages.BGPMessageKeepAlive{}
		if ok {
			peer.KeepAlive <- v
		}
	}
	return true, nil
}

func (s *BGPServer) ProcessSend(v interface{}, n *fgbgp.Neighbor) (bool, error) {
	log.Printf("ProcessSend: %v", v)
	return true, nil
}

func (s *BGPServer) ProcessUpdateEvent(e *messages.BGPMessageUpdate, n *fgbgp.Neighbor) (add bool) {
	peer, exists := s.GetPeerFromNeigh(n)
	if !exists {
		log.Println("PEER DOESN'T EXIST??", n.Addr.String(), n.PeerASN)
		return false
	}

	data := common.RouteData{}
	for _, v := range e.NLRI {
		prefix, ok := v.(messages.NLRI_IPPrefix)
		if ok {
			data.Prefixes = append(data.Prefixes, prefix.Prefix.String())
		}
	}

	for _, v := range e.PathAttributes {
		switch val := v.(type) {
		case messages.BGPAttribute_NEXTHOP:
			data.NextHop = val.NextHop.String()
		case messages.BGPAttribute_COMMUNITIES:
			for _, c := range val.Communities {
				data.Communities = append(data.Communities, []uint16{
					uint16(c / 65536), uint16(c % 65536),
				})
			}
		case messages.BGPAttribute_ORIGIN:
			data.Origin = int(val.Origin)
		case messages.BGPAttribute_ASPATH:
			data.AsPath = val.Segments[0].ASPath
		}
	}

	peer.RouteChannel <- &data
	return true
}

func (s *BGPServer) DisconnectedNeighbor(n *fgbgp.Neighbor) {
	peer, ok := s.GetPeerFromNeigh(n)
	if ok {
		peer.EOL <- true
		peer.EOL <- true
		peer.EOL <- true
		peer.EOL <- true
		peer.EOL <- true
	}
	log.Printf("DISCONNECTED %v\n", n)
}

func (s *BGPServer) NewNeighbor(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	log.Printf("GOT A NEW Neighbor %v %v\n", on, n)
	n.LocalHoldTime = time.Second * 60
	n.LocalEnableKeepAlive = false
	return true
}

func (s *BGPServer) OpenSend(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	log.Printf("OpenSend %v %v\n", on, n)
	return true
}

func (s *BGPServer) OpenConfirm() bool {
	log.Printf("OpenConfirm\n")
	return true
}

func CreateBGPServer(asn uint32, listenAddr string, identifier string) *BGPServer {
	manager := fgbgp.NewManager(asn, net.ParseIP(identifier), false, false)
	manager.UseDefaultUpdateHandler(10)
	server := &BGPServer{Fgbgp: manager, Peers: make(map[string]*Peer)}
	manager.SetEventHandler(server)
	manager.SetUpdateEventHandler(server)
	err := manager.NewServer(listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	manager.StartServers()

	return server
}
