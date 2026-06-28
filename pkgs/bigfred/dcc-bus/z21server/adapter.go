package z21server

import (
	"context"
	"errors"
	"net"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// Responder sends Z21 LAN replies to one handset client.
type Responder struct {
	client     *Client
	server     *Server
	subscribed map[uint16]struct{}
}

var _ remotes.ThrottleResponder = (*Responder)(nil)

// NewResponder adapts one Z21 client to remotes.ThrottleResponder.
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
		r.server.registry.SubscribeLoco(r.client.Key, addr)
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
	return r.server.writeUDP(&r.client.Addr, r.client.Key, pkt)
}

func (r *Responder) SendLocoError(ctx context.Context, addr uint16, code, detail string) error {
	_ = ctx
	_ = addr
	_ = code
	_ = detail
	return nil
}

// Adapter maps inbound Z21 packets to remotes.InboundDrivePort.
type Adapter struct {
	server *Server
	drive  remotes.InboundDrivePort
}

// NewAdapter wires the shared drive port into the Z21 server.
func NewAdapter(server *Server, drive remotes.InboundDrivePort) *Adapter {
	return &Adapter{server: server, drive: drive}
}

func (a *Adapter) throttleActor(client *Client) remotes.ThrottleActor {
	userID := uint(0)
	if p, ok := a.server.registry.Paired(client.Key); ok {
		userID = p.UserID
	}
	return remotes.ThrottleActor{
		UserID:    userID,
		SessionID: remotes.HandsetSessionID(client.Key),
	}
}

func (a *Adapter) driveScope(client *Client) remotes.DriveScope {
	p, ok := a.server.registry.Paired(client.Key)
	if !ok {
		return remotes.DriveScope{}
	}
	return remotes.DriveScope{
		AllowedAddrs:     p.AllowedAddrs,
		AllowAllVehicles: p.AllowAllVehicles,
	}
}

func (a *Adapter) authorize(client *Client, addr uint16) bool {
	p, ok := a.server.registry.Paired(client.Key)
	if !ok || a.drive == nil {
		return false
	}
	return a.drive.AuthorizeDrive(p.UserID, addr, a.driveScope(client))
}

func (a *Adapter) HandleSetLocoFunction(ctx context.Context, client *Client, pkt []byte) {
	addr, fn, on, ok := parseSetLocoFunction(pkt)
	if !ok {
		return
	}
	if !a.authorize(client, addr) {
		a.logDriveRejected(client, addr, "set_function")
		return
	}
	a.server.registry.SetLastActiveLoco(client.Key, addr)
	result := a.drive.SetFunction(ctx, a.throttleActor(client), NewResponder(a.server, client), contract.LocoSetFunctionWire{
		Address:  addr,
		Function: uint8(fn),
		On:       on,
	})
	if !result.OK {
		a.logDriveFailure(client, addr, "set_function", result.Code)
	}
}

func (a *Adapter) HandleSetLocoFunctionGroup(ctx context.Context, client *Client, pkt []byte) {
	addr, updates, ok := parseSetLocoFunctionGroup(pkt)
	if !ok || !a.authorize(client, addr) {
		if ok {
			a.logDriveRejected(client, addr, "set_function_group")
		}
		return
	}
	a.server.registry.SetLastActiveLoco(client.Key, addr)
	resp := NewResponder(a.server, client)
	actor := a.throttleActor(client)
	for _, u := range updates {
		result := a.drive.SetFunction(ctx, actor, resp, contract.LocoSetFunctionWire{
			Address:  addr,
			Function: uint8(u.fn),
			On:       u.on,
		})
		if !result.OK {
			a.logDriveFailure(client, addr, "set_function_group", result.Code)
			return
		}
	}
}

func (a *Adapter) HandleGetLocoInfo(ctx context.Context, client *Client, pkt []byte) {
	addr, ok := parseGetLocoInfo(pkt)
	if !ok {
		return
	}
	if !a.authorize(client, addr) {
		a.logDriveRejected(client, addr, "get_loco_info")
		return
	}
	a.server.registry.SetLastActiveLoco(client.Key, addr)
	result := a.drive.Subscribe(ctx, a.throttleActor(client), NewResponder(a.server, client), []uint16{addr})
	if !result.OK {
		a.logDriveFailure(client, addr, "get_loco_info", result.Code)
	}
}

func (a *Adapter) logDriveRejected(client *Client, addr uint16, action string) {
	if a.server.log == nil {
		return
	}
	a.server.log.WithFields(logrus.Fields{
		"client": client.Key,
		"loco":   addr,
		"action": action,
	}).Info("z21 drive rejected: not authorized")
}

func (a *Adapter) HandleSetLocoDrive(ctx context.Context, client *Client, pkt []byte) {
	addr, speed, forward, ok := parseSetLocoDrive(pkt)
	if !ok {
		return
	}
	if !a.authorize(client, addr) {
		a.logDriveRejected(client, addr, "set_speed")
		return
	}
	a.server.registry.SetLastActiveLoco(client.Key, addr)
	result := a.drive.SetSpeed(ctx, a.throttleActor(client), NewResponder(a.server, client), contract.LocoSetSpeedWire{
		Address: addr,
		Speed:   speed,
		Forward: forward,
	})
	if !result.OK {
		a.logDriveFailure(client, addr, "set_speed", result.Code)
	}
}

func (a *Adapter) logDriveFailure(client *Client, addr uint16, action, code string) {
	if a.server.log == nil {
		return
	}
	a.server.log.WithFields(logrus.Fields{
		"client": client.Key,
		"loco":   addr,
		"action": action,
		"code":   code,
	}).Info("z21 drive command failed")
}

func (a *Adapter) ReadLocoCV(addr uint16, cvNum commandstation.CVNum) (int, error) {
	if a.drive == nil {
		return 0, errors.New("z21server: no drive port")
	}
	return a.drive.ReadLocoCV(addr, cvNum)
}

func (a *Adapter) HandleSetBroadcastFlags(client *Client, pkt []byte) {
	if len(pkt) < 8 {
		return
	}
	a.server.applyBroadcastFlags(client, broadcastFlagsFromPkt(pkt))
}

// WriteTo sends a UDP packet to a remote handset (used in tests).
func (s *Server) WriteTo(addr *net.UDPAddr, pkt []byte) error {
	return s.writeUDP(addr, clientKey(addr), pkt)
}