package bgp

import (
	"context"
	"io"
	"log"
	"net"
	"time"

	"github.com/hamptonmoore/bgp.exposed/backend/common"
	gobgp "github.com/osrg/gobgp/v3/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GoBGPServer struct {
	Connection grpc.ClientConnInterface
	Server     gobgp.GobgpApiClient
}

func CreateGoBGPServer(host string, port string, bgpPort int32, routerID string, asn uint32) (GoBGPServer, error) {
	grpcOpts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock()}
	ctx := context.Background()
	target := net.JoinHostPort(host, port)
	cc, _ := context.WithTimeout(ctx, time.Second)

	conn, err := grpc.DialContext(cc, target, grpcOpts...)
	if err != nil {
		return GoBGPServer{}, err
	}
	server := gobgp.NewGobgpApiClient(conn)
	// server.StopBgp(ctx, &gobgp.StopBgpRequest{})
	server.StartBgp(ctx, &gobgp.StartBgpRequest{
		Global: &gobgp.Global{
			Asn:             asn,
			RouterId:        routerID,
			ListenPort:      bgpPort,
			ListenAddresses: []string{"0.0.0.0"},
		},
	})

	return GoBGPServer{
		Connection: conn,
		Server:     server,
	}, nil
}

type Downstream struct {
	Client gobgp.GobgpApiClient
	Create common.CreateRequest
}

func CreateDownstream(creation common.CreateRequest, server GoBGPServer) Downstream {
	client := gobgp.NewGobgpApiClient(server.Connection)
	down := Downstream{
		Client: client,
		Create: creation,
	}

	down.Client.AddPeer(context.Background(), &gobgp.AddPeerRequest{
		Peer: &gobgp.Peer{
			Conf: &gobgp.PeerConf{
				PeerAsn:         down.Create.PeerASN,
				LocalAsn:        down.Create.LocalASN,
				NeighborAddress: down.Create.PeerIP,
				RemovePrivate:   gobgp.RemovePrivate_REMOVE_NONE,
			},
		},
	})

	return down
}

func (d Downstream) SubscribeToPeer(channel chan *gobgp.Peer) error {
	watch, err := d.Client.WatchEvent(context.Background(), &gobgp.WatchEventRequest{
		Peer: &gobgp.WatchEventRequest_Peer{},
	})

	if err != nil {
		return err
	}

	for {
		r, err := watch.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if p := r.GetPeer(); p != nil && p.Type == gobgp.WatchEventResponse_PeerEvent_STATE {
			s := p.Peer
			log.Println("Got peer update")
			log.Println(s)
			if s.Conf.LocalAsn == d.Create.LocalASN && s.Conf.PeerAsn == d.Create.PeerASN && s.Conf.NeighborAddress == d.Create.PeerIP {
				channel <- s
			}
		}
	}
	return nil
}
