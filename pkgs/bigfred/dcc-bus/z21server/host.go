package z21server

import (
	"net"
	"strconv"
	"time"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotes/inbound"
	"github.com/dcc-bigfred/proto/go/pkgs/drive"
	z21proto "github.com/dcc-bigfred/proto/go/pkgs/z21"
)

var (
	_ drive.DriveHost       = (*Server)(nil)
	_ z21proto.DriveGate    = (*Server)(nil)
	_ z21proto.CVGate       = (*Server)(nil)
	_ z21proto.SessionHooks = (*Server)(nil)
)

func (s *Server) OnActivity(client drive.ClientID) {
	addr := udpAddrFromClientID(client)
	c := s.registry.Touch(addr, time.Now().UTC(), s.cfg.IPStickiness)
	ctx := s.ctx()
	if s.registry.NeedsSync(c.Key, sessionSyncStale) {
		s.syncPaired(ctx, c)
		s.registry.MarkSynced(c.Key)
	}
	if s.registry.IsPaired(c.Key) && s.cfg.Store != nil {
		s.registry.MarkSeenDirty(c.Key, contract.NowMS())
	}
	s.noteClientActivity(ctx, c)
}

func (s *Server) OnLogoff(client drive.ClientID) {
	s.evictClient(s.ctx(), string(client))
}

func (s *Server) OnBroadcastFlags(client drive.ClientID, flags uint32) {
	c := s.clientFor(client)
	if c == nil {
		return
	}
	s.registry.SetBroadcastFlags(c.Key, flags)
}

func (s *Server) Drive(client drive.ClientID, _ z21proto.DriveOp, _ uint16, pkt []byte) bool {
	c := s.clientFor(client)
	if c == nil {
		return true
	}
	s.handlePacket(s.ctx(), &c.Addr, pkt)
	return true
}

func (s *Server) CV(client drive.ClientID, op z21proto.CVOp, addr uint16, cvWire uint16, value uint8) (bool, []byte) {
	c := s.clientFor(client)
	if c == nil {
		return true, nil
	}
	ctx := s.ctx()
	remote := &c.Addr
	switch op {
	case z21proto.CVPomWrite:
		s.handlePOMWrite(ctx, remote, c, z21proto.BuildPomWriteByte(addr, cvWire, value))
	case z21proto.CVPomRead:
		s.handlePOMRead(ctx, remote, c, z21proto.BuildPomRead(addr, cvWire))
	case z21proto.CVProgWrite:
		s.handleProgTrackCVWrite(ctx, remote, c, z21proto.BuildProgWrite(cvWire, value))
	case z21proto.CVProgRead:
		s.handleProgTrackCVRead(ctx, remote, c, z21proto.BuildProgRead(cvWire))
	}
	return true, nil
}

func (s *Server) SetSpeed(drive.ClientID, uint16, uint8, bool, uint8) error { return nil }
func (s *Server) SetFunction(drive.ClientID, uint16, uint8, bool) error     { return nil }
func (s *Server) SetTrackPower(drive.ClientID, bool) error                  { return nil }
func (s *Server) Release(drive.ClientID, uint16)                            {}

func (s *Server) LocoState(addr uint16) (drive.LocoState, error) {
	if s.adapter != nil && s.adapter.drive != nil {
		snap := s.adapter.drive.LocoSnapshot(addr)
		return drive.LocoState{
			Addr:      addr,
			Speed:     snap.Speed,
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

func (s *Server) clientFor(id drive.ClientID) *Client {
	c, ok := s.registry.Get(string(id))
	if !ok {
		return nil
	}
	return c
}

func udpAddrFromClientID(id drive.ClientID) *net.UDPAddr {
	_, ep := inbound.ParseClientKey(string(id))
	host, port, err := net.SplitHostPort(ep)
	if err != nil {
		return &net.UDPAddr{IP: net.ParseIP(ep)}
	}
	p, _ := strconv.Atoi(port)
	return &net.UDPAddr{IP: net.ParseIP(host), Port: p}
}
