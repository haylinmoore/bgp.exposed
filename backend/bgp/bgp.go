package bgp

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/bgptools/fgbgp/messages"
	fgbgp "github.com/bgptools/fgbgp/server"
	"github.com/hamptonmoore/bgp.exposed/backend/common"
	log "github.com/sirupsen/logrus"
)

type Peer struct {
	Key              string
	PeerASN          uint32 `json:"peerASN"`
	PeerIP           string `json:"peerIP"`
	LocalASN         uint32 `json:"localASN"`
	Server           *BGPServer
	KeepAlive        chan *messages.BGPMessageKeepAlive
	Neighbor         *fgbgp.Neighbor
	SendChan         chan *common.Packet
	RoutesToAnnounce chan *common.RouteData
	Context          context.Context
	Cancel           context.CancelFunc
}

func (p *Peer) Log(msg string) {
	p.SendChan <- &common.Packet{
		Type: "FSMUpdate",
		Data: common.Event{
			Time:    uint64(time.Now().UTC().UnixNano()),
			Message: msg,
		},
	}
}

func (p *Peer) Handler() {
	// Wait for the peer to raise
	<-p.KeepAlive
	p.Log("recv-keepalive")
	p.Log("sent-keepalive")
main:
	for {
		select {
		case <-time.After(time.Second * 30):
			p.KeepAlive <- &messages.BGPMessageKeepAlive{}
		case <-p.KeepAlive:
			if p.Neighbor != nil {
				p.Neighbor.OutQueue <- messages.BGPMessageKeepAlive{}
				p.Log("sent-keepalive")
			}
		case route := <-p.RoutesToAnnounce:
			log.Println(route)
			announcement := &messages.BGPMessageUpdate{}
			if len(route.Withdraws) > 0 {
				for _, prefix := range route.Withdraws {
					_, pref, _ := net.ParseCIDR(prefix.Prefix)

					announcement.WithdrawnRoutes = append(announcement.WithdrawnRoutes, messages.NLRI_IPPrefix{
						Prefix: *pref,
						PathId: prefix.ID,
					})
				}
			}
			if len(route.Prefixes) > 0 {
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
				if len(route.Communities) > 0 {
					communities := []uint32{}
					for _, c := range route.Communities {
						communities = append(communities, uint32(c[1])+(uint32(c[0])*65536))
					}
					pa = append(pa, messages.BGPAttribute_COMMUNITIES{
						Communities: communities,
					})
				}

				announcement.PathAttributes = pa

				for _, prefix := range route.Prefixes {
					_, pref, _ := net.ParseCIDR(prefix.Prefix)

					announcement.NLRI = append(announcement.NLRI, messages.NLRI_IPPrefix{
						Prefix: *pref,
						PathId: prefix.ID,
					})
				}
			}
			p.Neighbor.OutQueue <- announcement
		case <-p.Context.Done():
			//log.Println(p.Neighbor.State.CurState)
			p.SendChan <- &common.Packet{
				Type: "FSMUpdate",
				Data: common.FSMUpdate{
					State: "Idle",
				},
			}
			p.Neighbor.Disconnect()
			p.Server.PeerLock.Lock()
			delete(p.Server.Peers, p.Key)
			p.Server.PeerLock.Unlock()
			break main
		}
	}
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

func (s *BGPServer) CreatePeer(request *common.CreateRequest, ctx context.Context, cancel context.CancelFunc) *Peer {
	log.Debugf("Creating peer %s", request.PeerIP)

	s.PeerLock.Lock()

	peer := &Peer{
		Key:              request.ToKey(),
		PeerASN:          request.PeerASN,
		LocalASN:         request.LocalASN,
		PeerIP:           request.PeerIP,
		Server:           s,
		SendChan:         make(chan *common.Packet, 1024),
		KeepAlive:        make(chan *messages.BGPMessageKeepAlive, 16),
		RoutesToAnnounce: make(chan *common.RouteData, 1024),
		Context:          ctx,
		Cancel:           cancel,
	}
	peer.SendChan <- &common.Packet{
		Type: "FSMUpdate",
		Data: common.FSMUpdate{
			State: "Idle",
		},
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
		key := n.Addr.String() + "|" + strconv.FormatUint(uint64(v.ASN), 10)
		s.PeerLock.Lock()
		peer, ok := s.Peers[key]
		s.PeerLock.Unlock()
		if ok {
			n.ASN = peer.LocalASN
			peer.Neighbor = n
			peer.SendChan <- &common.Packet{
				Type: "FSMUpdate",
				Data: common.FSMUpdate{
					State: "Active",
				},
			}
			return true, nil
		} else {
			n.Disconnect()
			return false, errors.New("no peer exists")
		}
	case *messages.BGPMessageKeepAlive:
		peer, ok := s.GetPeerFromNeigh(n)
		if ok {
			peer.Log("recv-keepalive")
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
	peer.Log("recv-update")

	data := common.RouteData{}
	for _, v := range e.NLRI {
		prefix, ok := v.(messages.NLRI_IPPrefix)
		if ok {
			data.Prefixes = append(data.Prefixes, common.NLRI{
				Prefix: prefix.Prefix.String(),
				ID:     prefix.PathId,
			})
		}
	}
	for _, v := range e.WithdrawnRoutes {
		prefix, ok := v.(messages.NLRI_IPPrefix)
		if ok {
			data.Withdraws = append(data.Prefixes, common.NLRI{
				Prefix: prefix.Prefix.String(),
				ID:     prefix.PathId,
			})
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

	peer.SendChan <- &common.Packet{
		Type: "RouteData",
		Data: data,
	}
	return true
}

func (s *BGPServer) DisconnectedNeighbor(n *fgbgp.Neighbor) {
	peer, ok := s.GetPeerFromNeigh(n)
	if ok {
		peer.SendChan <- &common.Packet{
			Type: "FSMUpdate",
			Data: common.FSMUpdate{
				State: "Idle",
			},
		}
	}
	log.Printf("DISCONNECTED %v\n", n)
}

func (s *BGPServer) NewNeighbor(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	n.LocalEnableKeepAlive = true
	peer, ok := s.GetPeerFromNeigh(n)
	if ok {
		peer.SendChan <- &common.Packet{
			Type: "FSMUpdate",
			Data: common.FSMUpdate{
				State:          "Established",
				HoldTimer:      uint(n.LocalHoldTime.Seconds()),
				KeepaliveTimer: uint(n.LocalHoldTime / time.Second / 3),
			},
		}
	}
	return true
}

func (s *BGPServer) OpenSend(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	log.Printf("OpenSend %v %v\n", on, n)
	peer, ok := s.GetPeerFromNeigh(n)
	if ok {
		peer.SendChan <- &common.Packet{
			Type: "FSMUpdate",
			Data: common.FSMUpdate{
				State: "OpenSent",
			},
		}
		return true
	}
	return false
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
