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
	RouteChannel     chan *messages.BGPMessageUpdate
	RoutesToAnnounce chan *common.RouteData
	EOL              chan bool
}

func (p *Peer) Handler() {
	// Wait for the peer to raise
	<-p.KeepAlive
main:
	for {
		select {
		case route := <-p.RoutesToAnnounce:
			pa := []messages.BGPAttributeIf{
				messages.BGPAttribute_ORIGIN{
					Origin: 1,
				},
				messages.BGPAttribute_NEXTHOP{NextHop: net.ParseIP(route.NextHop)},
				messages.BGPAttribute_ASPATH{Segments: []messages.ASPath_Segment{
					{
						SType:  2,
						ASPath: route.AsPath,
					},
				}},
			}

			announcement := &messages.BGPMessageUpdate{
				PathAttributes: pa,
			}

			_, pref, _ := net.ParseCIDR(route.Prefix)

			announcement.NLRI = append(announcement.NLRI, messages.NLRI_IPPrefix{
				Prefix: *pref,
				PathId: uint32(time.Now().UTC().Unix()),
			})

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
		RouteChannel:     make(chan *messages.BGPMessageUpdate, 1024),
		KeepAlive:        make(chan *messages.BGPMessageKeepAlive, 1),
		RoutesToAnnounce: make(chan *common.RouteData, 1024),
		EOL:              make(chan bool, 1),
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
	log.Printf("ProcessReceived: %s", msg)
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

	peer.RouteChannel <- e
	return true
}

func (s *BGPServer) DisconnectedNeighbor(n *fgbgp.Neighbor) {
	peer, ok := s.GetPeerFromNeigh(n)
	if ok {
		peer.EOL <- true
	}
	log.Printf("DISCONNECTED %v\n", n)
}

func (s *BGPServer) NewNeighbor(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	log.Printf("GOT A NEW Neighbor %v %v\n", on, n)
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
