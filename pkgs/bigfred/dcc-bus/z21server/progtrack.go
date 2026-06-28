package z21server

import (
	"context"
	"net"
)

// progTrackLoco is the virtual-CV map key for programming-track CV access (no loco address in packet).
const progTrackLoco uint16 = 0

func isProgTrackCVWrite(pkt []byte) bool {
	_, _, ok := parseProgTrackCVWrite(pkt)
	return ok
}

func isProgTrackCVRead(pkt []byte) bool {
	_, ok := parseProgTrackCVRead(pkt)
	return ok
}

// parseProgTrackCVWrite extracts CV wire index and value from LAN_X_CV_WRITE (§6.2).
func parseProgTrackCVWrite(pkt []byte) (cvWire int, value int, ok bool) {
	if len(pkt) < 10 {
		return 0, 0, false
	}
	_, header, okHdr := packetHeader(pkt)
	if !okHdr || header != HeaderXBus || pkt[4] != 0x24 || pkt[5] != 0x12 {
		return 0, 0, false
	}
	cvWire = int(pkt[6])<<8 | int(pkt[7])
	return cvWire, int(pkt[8]), true
}

// parseProgTrackCVRead extracts CV wire index from LAN_X_CV_READ (§6.1).
func parseProgTrackCVRead(pkt []byte) (cvWire int, ok bool) {
	if len(pkt) < 9 {
		return 0, false
	}
	_, header, okHdr := packetHeader(pkt)
	if !okHdr || header != HeaderXBus || pkt[4] != 0x23 || pkt[5] != 0x11 {
		return 0, false
	}
	cvWire = int(pkt[6])<<8 | int(pkt[7])
	return cvWire, true
}

func buildProgrammingModeReply() []byte {
	x := []byte{0x61, 0x02}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func (s *Server) handleProgTrackCVWrite(ctx context.Context, remote *net.UDPAddr, client *Client, pkt []byte) {
	cvWire, value, ok := parseProgTrackCVWrite(pkt)
	if !ok {
		return
	}
	if s.registry.IsPaired(client.Key) {
		_ = s.writeUDP(remote, client.Key, buildCVNackReply())
		return
	}

	s.registry.SetVirtualCV(client.Key, progTrackLoco, cvWire, byte(value))
	_ = s.writeUDP(remote, client.Key, buildProgrammingModeReply())

	if isPairingCVWire(cvWire) {
		if _, active := s.pairing.Handle(ctx, client, cvWire, value); active != nil {
			s.syncPaired(ctx, client)
			if s.log != nil {
				fields := pairingLogFields(active)
				fields["client"] = client.Key
				s.log.WithFields(fields).Info("z21 handset paired via CV3/CV4 (programming track)")
			}
		}
	}
	_ = s.writeUDP(remote, client.Key, buildCVResultReply(cvWire, byte(value)))
}

func (s *Server) handleProgTrackCVRead(ctx context.Context, remote *net.UDPAddr, client *Client, pkt []byte) {
	_ = ctx
	cvWire, ok := parseProgTrackCVRead(pkt)
	if !ok {
		return
	}
	if s.registry.IsPaired(client.Key) {
		_ = s.writeUDP(remote, client.Key, buildCVNackReply())
		return
	}

	value, found := s.registry.GetVirtualCV(client.Key, progTrackLoco, cvWire)
	if !found {
		value = 0
	}
	_ = s.writeUDP(remote, client.Key, buildProgrammingModeReply())
	_ = s.writeUDP(remote, client.Key, buildCVResultReply(cvWire, value))
}
