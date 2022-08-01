package bgp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/hamptonmoore/bgp.exposed/backend/common"
	gobgp "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/apiutil"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
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
	_, err = server.StartBgp(ctx, &gobgp.StartBgpRequest{
		Global: &gobgp.Global{
			Asn:             1001,
			RouterId:        routerID,
			ListenPort:      bgpPort,
			ListenAddresses: []string{"0.0.0.0"},
		},
	})

	return GoBGPServer{
		Connection: conn,
		Server:     server,
	}, err
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

	rd, _ := bgp.ParseRouteDistinguisher("0:0")
	v, _ := apiutil.MarshalRD(rd)

	_, err := down.Client.AddVrf(context.Background(), &gobgp.AddVrfRequest{
		Vrf: &gobgp.Vrf{
			Name: "100",
			Rd:   v,
			Id:   100,
		},
	})
	if err != nil {
		fmt.Println(err)
	}
	_, err = down.Client.AddPeer(context.Background(), &gobgp.AddPeerRequest{
		Peer: &gobgp.Peer{
			Conf: &gobgp.PeerConf{
				PeerAsn:         down.Create.PeerASN,
				LocalAsn:        down.Create.LocalASN,
				NeighborAddress: down.Create.PeerIP,
				RemovePrivate:   gobgp.RemovePrivate_REMOVE_NONE,
				Vrf:             "100",
			},
			State: &gobgp.PeerState{
				PeerAsn:  down.Create.PeerASN,
				LocalAsn: down.Create.LocalASN,
			},
			ApplyPolicy: &gobgp.ApplyPolicy{
				ImportPolicy: &gobgp.PolicyAssignment{
					Direction: gobgp.PolicyDirection_IMPORT,
					// Policies: []*gobgp.Policy{
					// 	{
					// 		Name: "AddImportCommunity",
					// 		Statements: []*gobgp.Statement{
					// 			{
					// 				Name: "AddImportCommunity",
					// 				Actions: &gobgp.Actions{
					// 					Community: &gobgp.CommunityAction{
					// 						Type:        gobgp.CommunityAction_ADD,
					// 						Communities: []string{"100:100"},
					// 					},
					// 				},
					// 			},
					// 		},
					// 	},
					// },
					DefaultAction: gobgp.RouteAction_ACCEPT,
				},
			},
		},
	})

	if err != nil {
		fmt.Println(err)
	}

	return down
}

func (d Downstream) SubscribeToPeer(channel chan gobgp.Peer) error {
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
				log.Println("Matches")
				channel <- *s
			}
		}
	}
	return nil
}
