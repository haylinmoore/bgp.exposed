package common

import "strconv"

type Packet struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"` // Either CreateRequest, UpdateRequest, RouteUpdate
}

type CreateRequest struct {
	PeerASN  uint32 `json:"peerASN"`
	PeerIP   string `json:"peerIP"`
	LocalASN uint32 `json:"localASN"`
}

func (c *CreateRequest) ToKey() string {
	return c.PeerIP + "|" + strconv.FormatUint(uint64(c.PeerASN), 10)
}

type UpdateRequest struct {
	FullTable   bool   `json:"fullTable"`
	AddPath     bool   `json:"addPath"`
	MD5Password string `json:"md5Password"`
}

type RouteData struct {
	Prefix         string   `json:"prefix"`
	AsPath         []uint32 `json:"asPath"`
	NextHop        string   `json:"nextHop"`
	Communities    [][]int  `json:"communities"`
	ExtCommunities [][]int  `json:"extCommunities"`
}

/* bi-directional route updates
The scope implies what end is making the change,
  scope = "server" means that the bgp.exposed backend will change the route(s) that it is announcing to the client
    frontend websocket -> backend
  scope = "client" the route(s) the client is sending over bgp have changed and should be reflected in the UI
    backend -> frontend websocket
*/
type RouteUpdate struct {
	Change string      `json:"change"`
	Scope  string      `json:"scope"`
	Routes []RouteData `json:"routes"`
}
