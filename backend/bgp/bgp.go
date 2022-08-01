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
	PeerASN      uint32 `json:"peerASN"`
	PeerIP       string `json:"peerIP"`
	LocalASN     uint32 `json:"localASN"`
	RouteChannel chan *messages.BGPMessageUpdate
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

func (s *BGPServer) CreatePeer(request *common.CreateRequest) chan *messages.BGPMessageUpdate {
	s.PeerLock.Lock()
	rc := make(chan *messages.BGPMessageUpdate, 128)
	s.Peers[request.ToKey()] = &Peer{
		PeerASN:      request.PeerASN,
		LocalASN:     request.LocalASN,
		PeerIP:       request.PeerIP,
		RouteChannel: rc,
	}
	s.PeerLock.Unlock()
	return rc
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
			return true, nil
		} else {
			return false, nil
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
	log.Printf("%v", n)
}

func (s *BGPServer) NewNeighbor(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	log.Printf("Neighbor %v %v", on, n)
	return true
}

func (s *BGPServer) OpenSend(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	log.Printf("%v %v", on, n)
	return true
}

func (s *BGPServer) OpenConfirm() bool {
	log.Printf("OpenConfirm")
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

func CreateRouteAnnouncement(prefixes []string, aspath []uint32, nexthop string) *messages.BGPMessageUpdate {

	pa := []messages.BGPAttributeIf{
		messages.BGPAttribute_ORIGIN{
			Origin: 1,
		},
		messages.BGPAttribute_NEXTHOP{NextHop: net.ParseIP(nexthop)},
		messages.BGPAttribute_ASPATH{Segments: []messages.ASPath_Segment{
			{
				SType:  2,
				ASPath: aspath,
			},
		}},
	}

	path := &messages.BGPMessageUpdate{
		PathAttributes: pa,
	}

	for i, prefix := range prefixes {
		_, pref, _ := net.ParseCIDR(prefix)

		path.NLRI = append(path.NLRI, messages.NLRI_IPPrefix{
			Prefix: *pref,
			PathId: uint32(time.Now().UTC().Unix()) + uint32(i),
		})
	}

	return path
}
