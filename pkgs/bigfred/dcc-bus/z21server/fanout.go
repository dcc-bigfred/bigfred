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
	for _, client := range s.registry.Snapshot() {
		if client.Paired == nil {
			continue
		}
		if client.BroadcastFlags&broadcastFlagDriving == 0 {
			continue
		}
		if !client.SubscribedTo(snap.Address) {
			continue
		}
		_ = s.writeUDP(&client.Addr, client.Key, pkt)
	}
}
