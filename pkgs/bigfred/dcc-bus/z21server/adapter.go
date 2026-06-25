package z21server

import (
	"context"
	"net"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/cmd"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// Responder sends Z21 LAN replies to one handset client.
type Responder struct {
	client     *Client
	server     *Server
	subscribed map[uint16]struct{}
}

// NewResponder adapts one Z21 client to cmd.Responder.
func NewResponder(server *Server, client *Client) *Responder {
	return &Responder{
		client:     client,
		server:     server,
		subscribed: make(map[uint16]struct{}, 4),
	}
}

func (r *Responder) Subscribe(addrs ...uint16) {
	for _, addr := range addrs {
		r.subscribed[addr] = struct{}{}
		r.client.SubscribeLoco(addr)
	}
}

func (r *Responder) SubscribedAddrs() []uint16 {
	out := make([]uint16, 0, len(r.subscribed))
	for addr := range r.subscribed {
		out = append(out, addr)
	}
	return out
}

func (r *Responder) SendLocoState(ctx context.Context, snap contract.LocoStateWire) error {
	_ = ctx
	pkt := buildLocoInfoReply(snap.Address, snap, r.server.cfg.SpeedSteps)
	return r.server.writeUDP(&r.client.Addr, pkt)
}

func (r *Responder) SendLocoError(ctx context.Context, addr uint16, code, detail string) error {
	_ = ctx
	_ = addr
	_ = code
	_ = detail
	return nil
}

func (r *Responder) SendAck(ctx context.Context, requestID string, payload protocol.AckPayload) error {
	_ = ctx
	_ = requestID
	_ = payload
	return nil
}

// Adapter maps inbound Z21 packets to cmd.Router handlers.
type Adapter struct {
	server *Server
	router *cmd.Router
}

// NewAdapter wires the shared command router into the Z21 server.
func NewAdapter(server *Server, router *cmd.Router) *Adapter {
	return &Adapter{server: server, router: router}
}

func (a *Adapter) actor(client *Client) cmd.Actor {
	userID := uint(0)
	if client.Paired != nil {
		userID = client.Paired.UserID
	}
	return cmd.Actor{
		UserID:    userID,
		SessionID: "z21:" + client.Key,
	}
}

func (a *Adapter) authorize(client *Client, addr uint16) bool {
	if client.Paired == nil || a.router == nil {
		return false
	}
	return a.router.AuthorizeZ21Drive(
		client.Paired.UserID,
		addr,
		client.Paired.AllowedAddrs,
		client.Paired.AllowAllVehicles,
	)
}

func (a *Adapter) HandleSetLocoDrive(ctx context.Context, client *Client, pkt []byte) {
	addr, speed, forward, ok := parseSetLocoDrive(pkt)
	if !ok || !a.authorize(client, addr) {
		return
	}
	resp := NewResponder(a.server, client)
	_ = a.router.HandleSetSpeed(ctx, a.actor(client), resp, contract.LocoSetSpeedWire{
		Address: addr,
		Speed:   speed,
		Forward: forward,
	}, "")
}

func (a *Adapter) HandleSetLocoFunction(ctx context.Context, client *Client, pkt []byte) {
	addr, fn, on, ok := parseSetLocoFunction(pkt)
	if !ok || !a.authorize(client, addr) {
		return
	}
	resp := NewResponder(a.server, client)
	_ = a.router.HandleSetFunction(ctx, a.actor(client), resp, contract.LocoSetFunctionWire{
		Address:  addr,
		Function: uint8(fn),
		On:       on,
	}, "")
}

func (a *Adapter) HandleGetLocoInfo(ctx context.Context, client *Client, pkt []byte) {
	addr, ok := parseGetLocoInfo(pkt)
	if !ok || !a.authorize(client, addr) {
		return
	}
	resp := NewResponder(a.server, client)
	_ = a.router.HandleSubscribe(ctx, a.actor(client), resp, protocol.LocoSubscribePayload{
		Addresses: []uint16{addr},
	}, "")
}

func (a *Adapter) HandleSetBroadcastFlags(client *Client, pkt []byte) {
	if len(pkt) < 8 {
		return
	}
	client.BroadcastFlags = uint32(pkt[4]) | uint32(pkt[5])<<8 | uint32(pkt[6])<<16 | uint32(pkt[7])<<24
}

// WriteTo sends a UDP packet to a remote handset (used in tests).
func (s *Server) WriteTo(addr *net.UDPAddr, pkt []byte) error {
	return s.writeUDP(addr, pkt)
}
