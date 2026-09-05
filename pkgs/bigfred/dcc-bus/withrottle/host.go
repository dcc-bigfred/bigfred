package withrottle

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
	"github.com/dcc-bigfred/proto/go/pkgs/drive"
	wtproto "github.com/dcc-bigfred/proto/go/pkgs/withrottle"
)

var (
	_ drive.DriveHost        = (*Server)(nil)
	_ drive.AcquireGate      = (*Server)(nil)
	_ drive.ActionGate       = (*Server)(nil)
	_ drive.ReleaseGate      = (*Server)(nil)
	_ drive.TrackPowerGate   = (*Server)(nil)
	_ drive.Subscriber       = (*Server)(nil)
	_ drive.SessionHooks     = (*Server)(nil)
	_ drive.NHook            = (*Server)(nil)
	_ wtproto.RosterProvider = (*Server)(nil)
	_ wtproto.LabelProvider  = (*Server)(nil)
)

func (s *Server) OnConnect(client drive.ClientID, deviceID string) {
	now := time.Now().UTC()
	key := string(client)
	if _, known := s.registry.Get(key); known {
		// Same HU on a new TCP connection while the previous one is still
		// registered: proto has already replaced the session, so per-connection
		// wire state (throttles, sentinel, pairing digits) must start fresh.
		if s.registry.IsPaired(key) {
			s.log.WithFields(logrus.Fields{
				"client":   key,
				"deviceId": deviceID,
			}).Warn("withrottle: paired handset device id taken over by new connection")
		}
		s.registry.ResetForNewConn(key)
	}
	c := s.registry.TouchByDeviceId(deviceID, nil, now)
	ctx := s.ctx()
	if s.registry.NeedsSync(c.Key, sessionSyncStale) {
		s.syncPaired(ctx, c)
		s.registry.MarkSynced(c.Key)
	}
	if s.cfg.Store != nil && s.registry.IsPaired(c.Key) {
		_ = s.cfg.Store.TouchSeen(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, c.Key, contract.NowMS(), contract.RemoteStickySessionIdle)
	}
}

func (s *Server) OnActivity(client drive.ClientID) {
	key := string(client)
	ctx := s.ctx()
	s.registry.touchLastSeen(key, time.Now().UTC())
	if s.registry.IsPaired(key) && s.cfg.Store != nil {
		s.registry.MarkSeenDirty(key, contract.NowMS())
	}
	if s.registry.NeedsSync(key, sessionSyncStale) {
		if c, ok := s.registry.Get(key); ok {
			s.syncPaired(ctx, c)
			s.registry.MarkSynced(key)
		}
	}
	s.noteClientActivity(ctx, key)
}

func (s *Server) OnQuit(client drive.ClientID) {
	key := string(client)
	s.quit.Store(key, struct{}{})
	s.evictClient(s.ctx(), key)
}

func (s *Server) OnDisconnect(client drive.ClientID) {
	key := string(client)
	if _, ok := s.quit.LoadAndDelete(key); ok {
		return
	}
	if p := s.protoServer(); p != nil && p.HasSession(client) {
		return
	}
	s.dropPresence(s.ctx(), key)
}

func (s *Server) OnN(client drive.ClientID, name string) bool {
	key := string(client)
	c, ok := s.registry.Get(key)
	if !ok {
		// N before HU: the session is still keyed by its TCP remote address.
		// v1 ignored it entirely (no burst, no heartbeat, no wire entry); the
		// initial burst goes out on HU once the paired roster is known.
		return true
	}
	s.registry.setDeviceName(key, name)
	if s.registry.IsPaired(key) {
		return false
	}
	consumed, active := s.pairing.HandleN(s.ctx(), c, name)
	if consumed && active != nil {
		fields := pairingLogFields(active)
		fields["client"] = key
		fields["pairingCode"] = active.PairingCode
		s.log.WithFields(fields).Info("withrottle handset paired via device name")
	}
	return consumed
}

func (s *Server) Acquire(client drive.ClientID, throttleID byte, addr uint16) (bool, []string) {
	key := string(client)
	c, ok := s.registry.Get(key)
	if !ok {
		return false, []string{"HMNot paired"}
	}
	paired := s.registry.IsPaired(key)
	if !paired {
		if !isSentinelAddr(addr, s.cfg.PairingAddr) {
			return false, []string{"HMNot paired"}
		}
		s.registry.withThrottle(key, throttleID, func(tw *throttleWire) {
			if tw.locos == nil {
				tw.locos = make(map[uint16]string)
			}
			tw.locos[addr] = locoKeyForAddr(addr)
			tw.lastLoco = addr
		})
		s.registry.setSentinelAcquired(key, true, throttleID)
		return false, buildSentinelAcquireReply(throttleID, addr)
	}
	if s.adapter == nil || !s.adapter.authorize(c, addr) {
		if s.adapter != nil {
			s.adapter.logDriveRejected(c, addr, "acquire")
		}
		return false, []string{"HMNot authorized"}
	}
	s.registry.withThrottle(key, throttleID, func(tw *throttleWire) {
		if tw.locos == nil {
			tw.locos = make(map[uint16]string)
		}
		tw.locos[addr] = locoKeyForAddr(addr)
		tw.lastLoco = addr
	})
	snap := contract.LocoStateWire{Forward: true}
	if s.adapter != nil && s.adapter.drive != nil {
		snap = s.adapter.drive.LocoSnapshot(addr)
	}
	return true, buildAcquireReply(throttleID, addr, s.functionsForAddr(addr), snap.Speed, snap.Forward, s.cfg.SpeedSteps)
}

func (s *Server) Subscribe(client drive.ClientID, addr uint16) error {
	if s.adapter == nil {
		return nil
	}
	c, ok := s.registry.Get(string(client))
	if !ok {
		return errNoClient
	}
	tid, _, okTid := s.registry.findThrottleForAddr(string(client), addr)
	if !okTid {
		tid = s.registry.sentinelThrottleID(string(client))
		if tid == 0 {
			tid = '0'
		}
	}
	resp := NewResponder(s, c, tid)
	result := s.adapter.drive.Subscribe(s.ctx(), s.adapter.throttleActor(c), resp, []uint16{addr})
	if result.OK {
		return nil
	}
	s.adapter.logDriveFailure(c, addr, "acquire", result.Code)
	s.registry.withThrottle(string(client), tid, func(tw *throttleWire) {
		delete(tw.locos, addr)
	})
	if result.Code != "" {
		return &subscribeError{code: result.Code}
	}
	return errSubscribeFailed
}

type subscribeError struct{ code string }

func (e *subscribeError) Error() string { return e.code }

var (
	errNoClient        = errString("withrottle: no client")
	errSubscribeFailed = errString("withrottle: subscribe failed")
)

type errString string

func (e errString) Error() string { return string(e) }

func (s *Server) GateRelease(client drive.ClientID, throttleID byte, locoKey string, addr uint16) bool {
	key := string(client)
	c, ok := s.registry.Get(key)
	if !ok {
		return true
	}
	paired := s.registry.IsPaired(key)
	if !paired && s.registry.sentinelAcquired(key) && isSentinelAddr(addr, s.cfg.PairingAddr) {
		s.registry.setSentinelAcquired(key, false, 0)
		s.registry.withThrottle(key, throttleID, func(tw *throttleWire) {
			delete(tw.locos, addr)
		})
		s.registry.ClearPairingBuffer(key)
		_ = s.writeLine(key, buildReleaseLine(throttleID, locoKey))
		return true
	}
	if !paired {
		return true
	}
	if s.adapter != nil {
		s.adapter.HandleRelease(s.ctx(), c, throttleID, locoKey, addr)
	}
	return true
}

func (s *Server) Action(client drive.ClientID, throttleID byte, locoKey string, addr uint16, prop string) bool {
	key := string(client)
	c, ok := s.registry.Get(key)
	if !ok {
		return true
	}
	if !s.registry.IsPaired(key) {
		s.handleUnpairedAction(c, throttleID, locoKey, addr, prop)
		return true
	}
	if s.adapter != nil {
		s.adapter.HandleAction(s.ctx(), c, throttleID, locoKey, addr, prop)
	}
	return true
}

func (s *Server) handleUnpairedAction(client *Client, throttleID byte, locoKey string, addr uint16, prop string) {
	if len(prop) == 0 {
		return
	}
	sentinel := isSentinelAddr(addr, s.cfg.PairingAddr) || (locoKey == "*" && s.registry.sentinelAcquired(client.Key) && s.throttleOnlySentinel(client.Key, throttleID))
	if sentinel && s.registry.sentinelAcquired(client.Key) {
		target := s.cfg.PairingAddr
		switch {
		case len(prop) >= 2 && prop[0] == 'V':
			if wireSpeed, estop, ok := parseSpeedValue(prop); ok {
				speed := uint8(0)
				if !estop {
					speed = dccSpeedFromWire(wireSpeed, s.cfg.SpeedSteps)
				}
				forward := s.virtual.Snapshot(client.Key, target).Forward
				s.sendVirtualLoco(client, throttleID, s.virtual.SetSpeed(client.Key, target, speed, forward))
			}
			return
		case len(prop) >= 2 && prop[0] == 'R':
			forward := prop[1] != '0'
			cur := s.virtual.Snapshot(client.Key, target)
			s.sendVirtualLoco(client, throttleID, s.virtual.SetSpeed(client.Key, target, cur.Speed, forward))
			return
		case len(prop) >= 2 && (prop[0] == 'F' || prop[0] == 'f'):
			if fn, on, force, ok := parseFunctionAction(prop); ok {
				s.sendVirtualLoco(client, throttleID, s.virtual.SetFunction(client.Key, target, fn, on))
				rising := s.registry.PairingFnRisingEdge(client.Key, fn, on)
				if pairingFnAccept(on, force, rising) {
					if consumed, active := s.pairing.HandleFn(s.ctx(), client, fn); consumed && active != nil {
						fields := pairingLogFields(active)
						fields["client"] = client.Key
						fields["pairingCode"] = active.PairingCode
						s.log.WithFields(fields).Info("withrottle handset paired via function keys")
					}
				}
			}
			return
		}
		return
	}
	if unpairedDriveProp(prop) && !sentinel {
		_ = s.writeLine(client.Key, "HMNot paired")
	}
}

func (s *Server) throttleOnlySentinel(key string, throttleID byte) bool {
	only := true
	sentinel := s.cfg.PairingAddr
	s.registry.withThrottle(key, throttleID, func(tw *throttleWire) {
		if len(tw.locos) == 0 {
			only = false
			return
		}
		for addr := range tw.locos {
			if !isSentinelAddr(addr, sentinel) {
				only = false
				return
			}
		}
	})
	return only
}

func unpairedDriveProp(prop string) bool {
	if len(prop) < 2 {
		return false
	}
	switch prop[0] {
	case 'V', 'R', 'F', 'f':
		return true
	default:
		return false
	}
}

func (s *Server) sendVirtualLoco(client *Client, throttleID byte, snap contract.LocoStateWire) {
	_ = NewResponder(s, client, throttleID).SendLocoState(s.ctx(), snap)
}

func (s *Server) TrackPower(drive.ClientID, bool) bool { return true }

func (s *Server) SetSpeed(drive.ClientID, uint16, uint8, bool, uint8) error { return nil }
func (s *Server) SetFunction(drive.ClientID, uint16, uint8, bool) error     { return nil }
func (s *Server) SetTrackPower(drive.ClientID, bool) error                  { return nil }
func (s *Server) Release(drive.ClientID, uint16)                            {}

func (s *Server) LocoState(addr uint16) (drive.LocoState, error) {
	if s.adapter != nil && s.adapter.drive != nil {
		snap := s.adapter.drive.LocoSnapshot(addr)
		return drive.LocoState{
			Addr:      addr,
			Speed:     uint8(wireSpeedFromDCC(snap.Speed, s.cfg.SpeedSteps)),
			Forward:   snap.Forward,
			Steps:     128,
			Functions: functionsMask(snap.Functions),
		}, nil
	}
	return drive.LocoState{Addr: addr, Forward: true, Steps: 128}, nil
}

func functionsMask(fns []bool) uint32 {
	var bits uint32
	for fn, on := range fns {
		if on && fn >= 0 && fn <= 31 {
			bits |= 1 << uint(fn)
		}
	}
	return bits
}

func (s *Server) Roster(client drive.ClientID) []wtproto.RosterEntry {
	key := string(client)
	paired := s.registry.IsPaired(key)
	sess, _ := s.registry.Session(key)
	if !paired {
		return []wtproto.RosterEntry{{
			Name: "Pair with BigFred",
			Addr: s.cfg.PairingAddr,
		}}
	}
	entries := rosterEntries(sess, s.allowedVehicles())
	out := make([]wtproto.RosterEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, wtproto.RosterEntry{Name: e.name, Addr: e.addr})
	}
	return out
}

func (s *Server) Labels(client drive.ClientID, addr uint16) []string {
	if !s.registry.IsPaired(string(client)) {
		return nil
	}
	return functionLabels(s.functionsForAddr(addr))
}

func (a *Adapter) logDriveRejected(client *Client, addr uint16, action string) {
	if a == nil || a.server.log == nil {
		return
	}
	a.server.log.WithFields(logrus.Fields{
		"client": client.Key,
		"loco":   addr,
		"action": action,
	}).Info("withrottle drive rejected: not authorized")
}

func (a *Adapter) logDriveFailure(client *Client, addr uint16, action, code string) {
	if a == nil || a.server.log == nil {
		return
	}
	a.server.log.WithFields(logrus.Fields{
		"client": client.Key,
		"loco":   addr,
		"action": action,
		"code":   code,
	}).Info("withrottle drive command failed")
}
