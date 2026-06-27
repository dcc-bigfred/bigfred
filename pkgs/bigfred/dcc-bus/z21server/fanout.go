package z21server

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

var _ remotes.LocoStateObserver = (*Server)(nil)

// LAN_SET_BROADCASTFLAGS bit 0: push LAN_X_LOCO_INFO for subscribed locos.
const broadcastFlagDriving uint32 = 0x00000001

// OnLocoStateChanged pushes LAN_X_LOCO_INFO to paired clients that subscribed
// to the address and enabled the driving broadcast flag.
func (s *Server) OnLocoStateChanged(ctx context.Context, snap contract.LocoStateWire) {
	_ = ctx
	if s.conn == nil || s.registry == nil {
		return
	}
	pkt := buildLocoInfoReply(snap.Address, snap, s.cfg.SpeedSteps)
	// Iterate only subscribers of this address instead of deep-copying
	// every registered client on each loco state change.
	for _, key := range s.registry.Subscribers(snap.Address) {
		client, ok := s.registry.Get(key)
		if !ok || client.Session == nil {
			continue
		}
		if s.registry.BroadcastFlags(key)&broadcastFlagDriving == 0 {
			continue
		}
		_ = s.writeUDP(&client.Addr, key, pkt)
	}
}
