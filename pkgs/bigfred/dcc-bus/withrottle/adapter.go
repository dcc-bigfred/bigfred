package withrottle

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

// Responder sends WiThrottle lines to one handset client.
type Responder struct {
	client     *Client
	server     *Server
	throttleID byte
	subscribed map[uint16]struct{}
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
	msg := code
	if detail != "" {
		msg = code + ": " + detail
	}
	return r.server.writeLine(r.client.Key, "HM"+msg)
}

// Adapter maps inbound WiThrottle lines to remotes.InboundDrivePort.
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

func (a *Adapter) HandleAcquire(ctx context.Context, client *Client, cmd MCommand) {
	addr, ok := parseAcquireAddr(cmd.LocoKey, cmd.Properties)
	if !ok {
		a.server.writeLine(client.Key, "HMInvalid acquire address")
		return
	}
	if !a.authorize(client, addr) {
		a.logDriveRejected(client, addr, "acquire")
		a.server.writeLine(client.Key, "HMNot authorized")
		return
	}
	key := locoKeyForAddr(addr)
	a.server.registry.withThrottle(client.Key, cmd.ThrottleID, func(tw *throttleWire) {
		tw.locos[addr] = key
		tw.lastLoco = addr
	})
	resp := NewResponder(a.server, client, cmd.ThrottleID)
	result := a.drive.Subscribe(ctx, a.throttleActor(client), resp, []uint16{addr})
	if !result.OK {
		a.logDriveFailure(client, addr, "acquire", result.Code)
		a.server.writeLine(client.Key, "HM"+result.Code)
		return
	}
	for _, line := range buildAcquireReply(cmd.ThrottleID, addr, a.server.functionsForAddr(addr)) {
		_ = a.server.writeLine(client.Key, line)
	}
}

func (a *Adapter) HandleRelease(ctx context.Context, client *Client, cmd MCommand) {
	var released []uint16
	a.server.registry.withThrottle(client.Key, cmd.ThrottleID, func(tw *throttleWire) {
		if cmd.LocoKey == "*" {
			released = make([]uint16, 0, len(tw.locos))
			for addr := range tw.locos {
				released = append(released, addr)
				delete(tw.locos, addr)
				if tw.lastSpeed != nil {
					delete(tw.lastSpeed, addr)
				}
			}
			return
		}
		if addr, _, ok := parseLocoKey(cmd.LocoKey); ok {
			delete(tw.locos, addr)
			if tw.lastSpeed != nil {
				delete(tw.lastSpeed, addr)
			}
			released = []uint16{addr}
		}
	})
	for _, addr := range released {
		a.server.registry.UnsubscribeLoco(client.Key, addr)
	}
	_ = ctx
	if cmd.LocoKey == "*" {
		a.server.writeLine(client.Key, "M"+string(cmd.ThrottleID)+"-*"+propSep+"r")
		return
	}
	a.server.writeLine(client.Key, buildReleaseLine(cmd.ThrottleID, cmd.LocoKey))
}

func (a *Adapter) HandleAction(ctx context.Context, client *Client, cmd MCommand) {
	if len(cmd.Properties) == 0 {
		return
	}
	prop := cmd.Properties[0]
	var addrs []uint16
	a.server.registry.withThrottle(client.Key, cmd.ThrottleID, func(tw *throttleWire) {
		addrs = addrFromLocoKey(cmd.LocoKey, tw)
	})
	if len(addrs) == 0 {
		return
	}
	switch {
	case len(prop) >= 2 && prop[0] == 'V':
		a.handleSpeed(ctx, client, cmd.ThrottleID, addrs, prop)
	case len(prop) >= 2 && prop[0] == 'R':
		a.handleDirection(ctx, client, cmd.ThrottleID, addrs, prop)
	case len(prop) >= 2 && (prop[0] == 'F' || prop[0] == 'f'):
		a.handleFunction(ctx, client, cmd.ThrottleID, addrs, prop)
	case prop == "X":
		a.handleEStop(ctx, client, addrs)
	case prop == "I":
		a.handleIdle(ctx, client, cmd.ThrottleID, addrs)
	case len(prop) >= 2 && prop[0] == 'q':
		a.handleQuery(ctx, client, cmd.ThrottleID, cmd.LocoKey, prop)
	case len(prop) >= 2 && prop[0] == 's':
		a.handleSpeedStepMode(client, cmd.ThrottleID, prop)
	default:
		// ignore unsupported sub-commands in v1
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

func (a *Adapter) logDriveRejected(client *Client, addr uint16, action string) {
	if a.server.log == nil {
		return
	}
	a.server.log.WithFields(logrus.Fields{
		"client": client.Key,
		"loco":   addr,
		"action": action,
	}).Info("withrottle drive rejected: not authorized")
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
	}).Info("withrottle drive command failed")
}

func parseAcquireAddr(locoKey string, props []string) (addr uint16, ok bool) {
	if addr, _, ok = parseLocoKey(locoKey); ok {
		return addr, true
	}
	if len(props) > 0 {
		if addr, _, ok = parseLocoKey(props[0]); ok {
			return addr, true
		}
		if addr, _, ok = parseLocoKey(props[len(props)-1]); ok {
			return addr, true
		}
	}
	return 0, false
}
