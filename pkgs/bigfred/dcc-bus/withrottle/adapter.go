package withrottle

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotes"
)

// Responder sends WiThrottle lines to one handset client.
type Responder struct {
	client         *Client
	server         *Server
	throttleID     byte
	subscribed     map[uint16]struct{}
	subscribeOrder []uint16
}

var _ remotes.ThrottleResponder = (*Responder)(nil)

// NewResponder adapts one WiThrottle client to remotes.ThrottleResponder.
func NewResponder(server *Server, client *Client, throttleID byte) *Responder {
	return &Responder{
		client:     client,
		server:     server,
		throttleID: throttleID,
		subscribed: make(map[uint16]struct{}, 4),
	}
}

func (r *Responder) Subscribe(addrs ...uint16) {
	for _, addr := range addrs {
		if _, ok := r.subscribed[addr]; ok {
			continue
		}
		r.subscribed[addr] = struct{}{}
		r.subscribeOrder = append(r.subscribeOrder, addr)
		r.server.registry.SubscribeLoco(r.client.Key, addr)
	}
}

func (r *Responder) Unsubscribe(addrs ...uint16) {
	for _, addr := range addrs {
		if _, ok := r.subscribed[addr]; !ok {
			continue
		}
		delete(r.subscribed, addr)
		r.subscribeOrder = removeSubscribeOrder(r.subscribeOrder, addr)
		r.server.registry.UnsubscribeLoco(r.client.Key, addr)
	}
}

func (r *Responder) OldestSubscribed() (uint16, bool) {
	for _, addr := range r.subscribeOrder {
		if _, ok := r.subscribed[addr]; ok {
			return addr, true
		}
	}
	return 0, false
}

func removeSubscribeOrder(order []uint16, addr uint16) []uint16 {
	out := order[:0]
	for _, a := range order {
		if a != addr {
			out = append(out, a)
		}
	}
	return out
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
	lines := buildLocoNotify(r.throttleID, locoKeyForAddr(snap.Address), snap, r.server.cfg.SpeedSteps)
	for _, line := range lines {
		if err := r.server.writeLine(r.client.Key, line); err != nil {
			return err
		}
	}
	return nil
}

func (r *Responder) SendLocoError(ctx context.Context, addr uint16, code, detail string) error {
	_ = ctx
	_ = addr
	return r.server.writeHM(r.client.Key, joinHM(code, detail))
}

// Adapter maps inbound WiThrottle actions to remotes.InboundDrivePort.
type Adapter struct {
	server *Server
	drive  remotes.InboundDrivePort
}

// NewAdapter wires the shared drive port into the WiThrottle server.
func NewAdapter(server *Server, drive remotes.InboundDrivePort) *Adapter {
	return &Adapter{server: server, drive: drive}
}

func (a *Adapter) throttleActor(client *Client) remotes.ThrottleActor {
	userID := uint(0)
	if p, ok := a.server.registry.Session(client.Key); ok {
		userID = p.UserID
	}
	return remotes.ThrottleActor{
		UserID:    userID,
		SessionID: remotes.HandsetSessionID(client.Key),
		Source:    "withrottle",
	}
}

func (a *Adapter) driveScope(client *Client) remotes.DriveScope {
	p, ok := a.server.registry.Session(client.Key)
	if !ok {
		return remotes.DriveScope{}
	}
	return remotes.DriveScope{
		AllowedAddrs:     p.AllowedAddrs,
		AllowAllVehicles: p.AllowAllVehicles,
	}
}

func (a *Adapter) authorize(client *Client, addr uint16) bool {
	p, ok := a.server.registry.Session(client.Key)
	if !ok || a.drive == nil {
		return false
	}
	return a.drive.AuthorizeDrive(p.UserID, addr, a.driveScope(client))
}

func (a *Adapter) HandleRelease(ctx context.Context, client *Client, throttleID byte, locoKey string, addr uint16) {
	_ = ctx
	released := []uint16{addr}
	if locoKey == "*" {
		a.server.registry.withThrottle(client.Key, throttleID, func(tw *throttleWire) {
			released = make([]uint16, 0, len(tw.locos))
			for a := range tw.locos {
				released = append(released, a)
				delete(tw.locos, a)
				if tw.lastSpeed != nil {
					delete(tw.lastSpeed, a)
				}
			}
		})
	} else {
		a.server.registry.withThrottle(client.Key, throttleID, func(tw *throttleWire) {
			delete(tw.locos, addr)
			if tw.lastSpeed != nil {
				delete(tw.lastSpeed, addr)
			}
		})
	}
	for _, aaddr := range released {
		a.server.registry.UnsubscribeLoco(client.Key, aaddr)
		if a.drive != nil {
			a.drive.Release(a.throttleActor(client), aaddr)
		}
	}
	if locoKey == "*" {
		_ = a.server.writeLine(client.Key, "M"+string(throttleID)+"-*"+propSep+"r")
		return
	}
	_ = a.server.writeLine(client.Key, buildReleaseLine(throttleID, locoKey))
}

func (a *Adapter) HandleAction(ctx context.Context, client *Client, throttleID byte, locoKey string, addr uint16, prop string) {
	if len(prop) == 0 {
		return
	}
	addrs := []uint16{addr}
	if locoKey == "*" {
		a.server.registry.withThrottle(client.Key, throttleID, func(tw *throttleWire) {
			addrs = make([]uint16, 0, len(tw.locos))
			for a := range tw.locos {
				addrs = append(addrs, a)
			}
		})
	}
	if len(addrs) == 0 {
		return
	}
	switch {
	case len(prop) >= 2 && prop[0] == 'V':
		a.handleSpeed(ctx, client, throttleID, addrs, prop)
	case len(prop) >= 2 && prop[0] == 'R':
		a.handleDirection(ctx, client, throttleID, addrs, prop)
	case len(prop) >= 2 && (prop[0] == 'F' || prop[0] == 'f'):
		a.handleFunction(ctx, client, throttleID, addrs, prop)
	case prop == "X":
		a.handleEStop(ctx, client, addrs)
	case prop == "I":
		a.handleIdle(ctx, client, throttleID, addrs)
	case len(prop) >= 2 && prop[0] == 'q':
		a.handleQuery(ctx, client, throttleID, locoKey, prop)
	case len(prop) >= 2 && prop[0] == 's':
		a.handleSpeedStepMode(client, throttleID, prop)
	}
}

func (a *Adapter) handleSpeed(ctx context.Context, client *Client, throttleID byte, addrs []uint16, prop string) {
	wireSpeed, estop, ok := parseSpeedValue(prop)
	if !ok {
		return
	}
	resp := NewResponder(a.server, client, throttleID)
	actor := a.throttleActor(client)
	forward := true
	a.server.registry.withThrottle(client.Key, throttleID, func(tw *throttleWire) {
		forward = tw.forward
	})
	for _, addr := range addrs {
		if !a.authorize(client, addr) {
			a.logDriveRejected(client, addr, "set_speed")
			_ = resp.SendLocoError(ctx, addr, "Not authorized", "")
			continue
		}
		if estop || wireSpeed == 1 {
			session := remotes.HandsetSession{ClientKey: client.Key, UserID: actor.UserID}
			a.drive.ApplyHandsetPilotEStop(ctx, session, addr)
			continue
		}
		speed := dccSpeedFromWire(wireSpeed, a.server.cfg.SpeedSteps)
		result := a.drive.SetSpeed(ctx, actor, resp, contract.LocoSetSpeedWire{
			Address: addr,
			Speed:   speed,
			Forward: forward,
		})
		if result.OK {
			a.server.registry.setLastSpeed(client.Key, throttleID, addr, speed)
		} else {
			a.logDriveFailure(client, addr, "set_speed", result.Code)
		}
	}
}

func (a *Adapter) handleDirection(ctx context.Context, client *Client, throttleID byte, addrs []uint16, prop string) {
	forward := len(prop) >= 2 && prop[1] != '0'
	a.server.registry.withThrottle(client.Key, throttleID, func(tw *throttleWire) {
		tw.forward = forward
	})
	resp := NewResponder(a.server, client, throttleID)
	actor := a.throttleActor(client)
	for _, addr := range addrs {
		if !a.authorize(client, addr) {
			a.logDriveRejected(client, addr, "set_direction")
			_ = resp.SendLocoError(ctx, addr, "Not authorized", "")
			continue
		}
		speed, ok := a.server.registry.lastSpeed(client.Key, throttleID, addr)
		if !ok {
			speed = 0
		}
		result := a.drive.SetSpeed(ctx, actor, resp, contract.LocoSetSpeedWire{
			Address: addr,
			Speed:   speed,
			Forward: forward,
		})
		if !result.OK {
			a.logDriveFailure(client, addr, "set_direction", result.Code)
		}
	}
}

func (a *Adapter) handleFunction(ctx context.Context, client *Client, throttleID byte, addrs []uint16, prop string) {
	fn, on, _, ok := parseFunctionAction(prop)
	if !ok {
		return
	}
	resp := NewResponder(a.server, client, throttleID)
	actor := a.throttleActor(client)
	for _, addr := range addrs {
		if !a.authorize(client, addr) {
			a.logDriveRejected(client, addr, "set_function")
			_ = resp.SendLocoError(ctx, addr, "Not authorized", "")
			continue
		}
		result := a.drive.SetFunction(ctx, actor, resp, contract.LocoSetFunctionWire{
			Address:  addr,
			Function: uint8(fn),
			On:       on,
		})
		if !result.OK {
			a.logDriveFailure(client, addr, "set_function", result.Code)
		}
	}
}

func (a *Adapter) handleEStop(ctx context.Context, client *Client, addrs []uint16) {
	actor := a.throttleActor(client)
	session := remotes.HandsetSession{ClientKey: client.Key, UserID: actor.UserID}
	for _, addr := range addrs {
		if !a.authorize(client, addr) {
			continue
		}
		a.drive.ApplyHandsetPilotEStop(ctx, session, addr)
	}
}

func (a *Adapter) handleIdle(ctx context.Context, client *Client, throttleID byte, addrs []uint16) {
	resp := NewResponder(a.server, client, throttleID)
	actor := a.throttleActor(client)
	forward := true
	a.server.registry.withThrottle(client.Key, throttleID, func(tw *throttleWire) {
		forward = tw.forward
	})
	for _, addr := range addrs {
		if !a.authorize(client, addr) {
			continue
		}
		_ = a.drive.SetSpeed(ctx, actor, resp, contract.LocoSetSpeedWire{
			Address: addr,
			Speed:   0,
			Forward: forward,
		})
		a.server.registry.setLastSpeed(client.Key, throttleID, addr, 0)
	}
}

func (a *Adapter) handleQuery(ctx context.Context, client *Client, throttleID byte, locoKey, prop string) {
	_ = ctx
	forward := true
	a.server.registry.withThrottle(client.Key, throttleID, func(tw *throttleWire) {
		forward = tw.forward
	})
	switch prop {
	case "qR":
		dir := 0
		if forward {
			dir = 1
		}
		_ = a.server.writeLine(client.Key, fmt.Sprintf("M%cA%s%sR%d", throttleID, locoKey, propSep, dir))
	case "qV":
		addr, _, ok := parseLocoKey(locoKey)
		if !ok {
			return
		}
		speed, ok := a.server.registry.lastSpeed(client.Key, throttleID, addr)
		wireSpeed := 0
		if ok {
			wireSpeed = wireSpeedFromDCC(speed, a.server.cfg.SpeedSteps)
		}
		_ = a.server.writeLine(client.Key, fmt.Sprintf("M%cA%s%sV%d", throttleID, locoKey, propSep, wireSpeed))
	}
}

func (a *Adapter) handleSpeedStepMode(client *Client, throttleID byte, prop string) {
	mode, err := strconv.Atoi(prop[1:])
	if err != nil {
		return
	}
	a.server.registry.withThrottle(client.Key, throttleID, func(tw *throttleWire) {
		tw.speedSteps = mode
	})
}

func (s *Server) writeHM(key, msg string) error {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil
	}
	if len(msg) > maxHMPayload {
		msg = truncateUTF8(msg, maxHMPayload)
	}
	return s.writeLine(key, "HM"+msg)
}

func joinHM(code, detail string) string {
	code = strings.TrimSpace(code)
	detail = strings.TrimSpace(detail)
	switch {
	case code == "" && detail == "":
		return ""
	case detail == "":
		return code
	case code == "":
		return detail
	default:
		return code + ": " + detail
	}
}

const maxHMPayload = 64

func truncateUTF8(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	s = s[:max]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
