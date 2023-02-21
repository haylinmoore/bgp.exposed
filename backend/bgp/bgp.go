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
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

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

func (p *Peer) ToKey() string {
	return p.PeerIP + "|" + strconv.FormatUint(uint64(p.PeerASN), 10)
}

func (p *Peer) Handler() {
	// Wait for the peer to raise
	log.Tracef("[Handler %s] Waiting for peer to come up", p.ToKey())
	<-p.KeepAlive

	log.Tracef("[Handler %s] Peer came up", p.ToKey())

	p.Log("recv-keepalive")
	p.Log("sent-keepalive")
main:
	for {
		select {
		case <-p.Context.Done():
			log.Tracef("[Handler %s] Websocket closed", p.ToKey())

			p.SendChan <- &common.Packet{
				Type: "FSMUpdate",
				Data: common.FSMUpdate{
					State: "Idle",
				},
			}
			if p.Neighbor != nil {
				p.Neighbor.Disconnect()
			}
			p.Server.PeerLock.Lock()
			delete(p.Server.Peers, p.Key)
			p.Server.PeerLock.Unlock()
			log.Tracef("[Handler %s] Peer deleted", p.ToKey())
			break main
		case <-time.After(time.Second * 30):
			p.KeepAlive <- &messages.BGPMessageKeepAlive{}
		case <-p.KeepAlive:
			log.Tracef("[Handler %s] Sending KEEPALIVE", p.ToKey())
			if p.Neighbor != nil {
				p.Neighbor.OutQueue <- messages.BGPMessageKeepAlive{}
				p.Log("sent-keepalive")
			}
		case route := <-p.RoutesToAnnounce:
			announcement := &messages.BGPMessageUpdate{}
			if len(route.Withdraws) > 0 {
				log.Tracef("[Handler %s] Withdrawing routes: %+v", p.ToKey(), route.Withdraws)
				for _, prefix := range route.Withdraws {
					_, pref, _ := net.ParseCIDR(prefix.Prefix)

					announcement.WithdrawnRoutes = append(announcement.WithdrawnRoutes, messages.NLRI_IPPrefix{
						Prefix: *pref,
						PathId: prefix.ID,
					})
				}
			}
			if len(route.Prefixes) > 0 {
				log.Tracef("[Handler %s] Announcing routes: %+v", p.ToKey(), route.Prefixes)
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
				if len(route.LargeCommunities) > 0 {
					pa = append(pa, messages.BGPAttribute_LARGECOMMUNITIES{
						Communities: route.LargeCommunities,
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
		}
	}
}

type BGPServer struct {
	Fgbgp    *fgbgp.Manager
	PeerLock sync.RWMutex
	Peers    map[string]*Peer
}

func neighborToKey(n *fgbgp.Neighbor) string {
	return n.Addr.String() + "|" + strconv.FormatUint(uint64(n.PeerASN), 10)
}

func (s *BGPServer) GetPeerFromNeigh(n *fgbgp.Neighbor) (*Peer, bool) {
	s.PeerLock.Lock()
	defer s.PeerLock.Unlock()

	peer, ok := s.Peers[neighborToKey(n)]
	return peer, ok
}

func (s *BGPServer) CreatePeer(request *common.CreateRequest, ctx context.Context, cancel context.CancelFunc) (*Peer, error) {
	log.Tracef("[CreatePeer] Creating peer %+v", request)

	s.PeerLock.Lock()
	defer s.PeerLock.Unlock()

	existingPeer, exists := s.Peers[request.ToKey()]
	if exists {
		log.Debugf("[CreatePeer] Peer with key %s already exists: %+v", request.ToKey(), existingPeer)
		return nil, errors.New("Peer already exists")
	}

	peer := &Peer{
		Key:              request.ToKey(),
		PeerASN:          request.PeerASN,
		LocalASN:         request.LocalASN,
		PeerIP:           request.PeerIP,
		Server:           s,
		SendChan:         make(chan *common.Packet, 512),
		KeepAlive:        make(chan *messages.BGPMessageKeepAlive, 512),
		RoutesToAnnounce: make(chan *common.RouteData, 512),
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

	log.Tracef("[CreatePeer] Peer created successfully %+v", request)

	return peer, nil
}

func (s *BGPServer) Notification(msg *messages.BGPMessageNotification, n *fgbgp.Neighbor) bool {
	log.Debugf("[Notification %s] Received NOTIFICATION message: %+v", neighborToKey(n), msg)
	return true
}

func (s *BGPServer) ProcessReceived(msg interface{}, n *fgbgp.Neighbor) (bool, error) {
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
			log.Debugf("[ProcessReceived %s] Received OPEN message: %+v", neighborToKey(n), msg)
			peer.SendChan <- &common.Packet{
				Type: "FSMUpdate",
				Data: common.FSMUpdate{
					State: "Active",
				},
			}
			return true, nil
		} else {
			n.PeerASN = uint32(v.ASN)
			log.Debugf("[ProcessReceived %s] Received OPEN message for nonexistent peer: %+v", neighborToKey(n), msg)
			n.Disconnect()
			return false, errors.New("no peer exists")
		}
	case *messages.BGPMessageKeepAlive:
		peer, ok := s.GetPeerFromNeigh(n)
		if ok {
			log.Tracef("[ProcessReceived %s] Received KEEPALIVE message", neighborToKey(n))
			peer.Log("recv-keepalive")
			peer.KeepAlive <- v
		} else {
			log.Errorf("[ProcessReceived %s] Received KEEPALIVE message for nonexistent peer???", neighborToKey(n))
		}
	}
	return true, nil
}

func (s *BGPServer) ProcessSend(v interface{}, n *fgbgp.Neighbor) (bool, error) {
	// since we're passive, does this ever get called?
	log.Debugf("[ProcessSend %s]: %v", neighborToKey(n), v)
	return true, nil
}

func (s *BGPServer) ProcessUpdateEvent(e *messages.BGPMessageUpdate, n *fgbgp.Neighbor) (add bool) {
	peer, exists := s.GetPeerFromNeigh(n)
	if !exists {
		log.Errorf("[ProcessUpdateEvent %s] Got UPDATE message for nonexistent peer???", neighborToKey(n))
		return false
	}

	log.Debugf("[ProcessUpdateEvent %s] Got UPDATE message. Adding prefixes %v, removing prefixes %v, with attributes %v", neighborToKey(n), e.NLRI, e.WithdrawnRoutes, e.PathAttributes)
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
		case messages.BGPAttribute_LARGECOMMUNITIES:
			for _, c := range val.Communities {
				data.LargeCommunities = append(data.LargeCommunities, c)
			}
		case messages.BGPAttribute_ORIGIN:
			data.Origin = int(val.Origin)
		case messages.BGPAttribute_ASPATH:
			data.AsPath = val.Segments[0].ASPath
		}
	}

	log.Tracef("[ProcessUpdateEvent %s] Sending RouteData to client", neighborToKey(n))
	peer.SendChan <- &common.Packet{
		Type: "RouteData",
		Data: data,
	}
	return true
}

func (s *BGPServer) DisconnectedNeighbor(n *fgbgp.Neighbor) {
	peer, ok := s.GetPeerFromNeigh(n)
	if ok {
		log.Infof("[DisconnectedNeighbor %s] Neighbor is down", neighborToKey(n))
		peer.SendChan <- &common.Packet{
			Type: "FSMUpdate",
			Data: common.FSMUpdate{
				State: "Idle",
			},
		}
	} else {
		log.Debugf("[DisconnectedNeighbor %s] Disconnected neighbor for nonexistent peer", neighborToKey(n))
	}
}

func (s *BGPServer) NewNeighbor(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	n.LocalEnableKeepAlive = true
	peer, ok := s.GetPeerFromNeigh(n)
	if ok {
		log.Infof("[NewNeighbor %s] Neighbor is up", neighborToKey(n))
		peer.SendChan <- &common.Packet{
			Type: "FSMUpdate",
			Data: common.FSMUpdate{
				State:          "Established",
				HoldTimer:      uint(n.LocalHoldTime.Seconds()),
				KeepaliveTimer: uint(n.LocalHoldTime / time.Second / 3),
			},
		}
	} else {
		log.Errorf("[NewNeighbor %s] Got neighbor establishment for nonexistent peer???", neighborToKey(n))
	}
	return true
}

func (s *BGPServer) OpenSend(on *messages.BGPMessageOpen, n *fgbgp.Neighbor) bool {
	// since we're passive, does this ever get called?
	peer, ok := s.GetPeerFromNeigh(n)
	if ok {
		log.Debugf("[OpenSend %s] sent message %+v", neighborToKey(n), on)
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

func CreateBGPServer(asn uint32, listenAddr string, identifier string, logger *logrus.Logger) *BGPServer {
	log = logger
	
	log.Tracef("[CreateBGPServer] creating fgbgp manager")
	manager := fgbgp.NewManager(asn, net.ParseIP(identifier), false, false)
	manager.UseDefaultUpdateHandler(10)
	server := &BGPServer{Fgbgp: manager, Peers: make(map[string]*Peer)}
	manager.SetEventHandler(server)
	manager.SetUpdateEventHandler(server)

	log.Tracef("[CreateBGPServer] creating fgbgp server with listenAddr %s", listenAddr)
	err := manager.NewServer(listenAddr)
	if err != nil {
		log.Fatalf("[CreateBGPServer] failed creating fgbgp server: %s", err)
	}
	log.Tracef("[CreateBGPServer] starting fgbgp server")
	manager.StartServers()

	return server
}
