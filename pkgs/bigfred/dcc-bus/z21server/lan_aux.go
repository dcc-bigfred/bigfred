package z21server

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"

	"github.com/sirupsen/logrus"
)

func (s *Server) handleCommonLAN(ctx context.Context, remote *net.UDPAddr, client *Client, pkt []byte) bool {
	_, header, ok := packetHeader(pkt)
	if !ok {
		return false
	}
	switch header {
	case HeaderRMBusGetData:
		if len(pkt) < 5 {
			return false
		}
		reply := buildRMBusDataReply(pkt[4])
		_ = s.writeUDP(remote, client.Key, reply)
		return true
	case HeaderGetLocoMode:
		if len(pkt) < 6 {
			return false
		}
		addr := binary.BigEndian.Uint16(pkt[4:6])
		reply := buildLocoModeReply(addr)
		_ = s.writeUDP(remote, client.Key, reply)
		return true
	case HeaderLocoNetFromLAN:
		s.logLocoNetFromLAN(client.Key, pkt)
		return true
	case HeaderLanKeepalive, HeaderLanSessionProbe:
		_ = s.writeUDP(remote, client.Key, buildAppKeepaliveReply(pkt))
		return true
	default:
		_ = ctx
		return false
	}
}

func buildRMBusDataReply(group byte) []byte {
	data := make([]byte, 11)
	data[0] = group
	return buildReply(HeaderRMBusDataChanged, data)
}

// buildAppKeepaliveReply answers the Roco mobile app LAN 0x0035/0x0036 probes.
// The app keeps sending them until it receives LAN_X_STATUS_CHANGED.
func buildAppKeepaliveReply(_ []byte) []byte {
	return buildStatusChangedReply()
}

func buildLocoModeReply(locoAddr uint16) []byte {
	data := make([]byte, 3)
	binary.BigEndian.PutUint16(data[0:2], locoAddr)
	data[2] = 0 // DCC
	return buildReply(HeaderGetLocoMode, data)
}

func (s *Server) logLocoNetFromLAN(clientKey string, pkt []byte) {
	if s.log == nil {
		return
	}
	payload := ""
	if len(pkt) > 4 {
		payload = hex.EncodeToString(pkt[4:])
	}
	s.log.WithFields(logrus.Fields{
		"client":  clientKey,
		"packet":  "LAN_LOCONET_FROM_LAN",
		"len":     len(pkt),
		"payload": payload,
		"raw":     hex.EncodeToString(pkt),
	}).Info("z21 loconet from lan")
}

func (s *Server) handlePOMWrite(ctx context.Context, remote *net.UDPAddr, client *Client, pkt []byte) {
	loco, cvWire, value, ok := parsePOMWriteByte(pkt)
	if !ok {
		return
	}
	if s.registry.IsPaired(client.Key) {
		if s.log != nil {
			s.log.WithFields(logrus.Fields{
				"client": client.Key,
				"loco":   loco,
				"cvWire": cvWire,
			}).Info("z21 pom write ignored when paired")
		}
		_ = s.writeUDP(remote, client.Key, buildCVNackReply())
		return
	}

	s.registry.SetVirtualCV(client.Key, loco, cvWire, byte(value))
	_ = s.writeUDP(remote, client.Key, buildCVResultReply(cvWire, byte(value)))
	if isPairingCVWire(cvWire) {
		if _, active := s.pairing.Handle(ctx, client, cvWire, value); active != nil {
			s.syncPaired(ctx, client)
			s.clearVirtualLoco(client.Key)
			fields := pairingLogFields(active)
			fields["client"] = client.Key
			s.log.WithFields(fields).Info("z21 handset paired via CV3/CV4")
		}
	}
}

func (s *Server) handlePOMRead(ctx context.Context, remote *net.UDPAddr, client *Client, pkt []byte) {
	_ = ctx
	loco, cvWire, ok := parsePOMReadByte(pkt)
	if !ok {
		return
	}

	if !s.registry.IsPaired(client.Key) {
		value, found := s.registry.GetVirtualCV(client.Key, loco, cvWire)
		if !found {
			value = 0
		}
		_ = s.writeUDP(remote, client.Key, buildCVResultReply(cvWire, value))
		return
	}

	if s.adapter == nil || !s.adapter.authorize(client, loco) {
		_ = s.writeUDP(remote, client.Key, buildCVNackReply())
		return
	}

	v, err := s.adapter.ReadLocoCV(loco, cvWireToNum(cvWire))
	if err != nil {
		if s.log != nil {
			s.log.WithError(err).WithFields(logrus.Fields{
				"client": client.Key,
				"loco":   loco,
				"cvWire": cvWire,
			}).Info("z21 pom read from loco failed")
		}
		_ = s.writeUDP(remote, client.Key, buildCVNackReply())
		return
	}
	_ = s.writeUDP(remote, client.Key, buildCVResultReply(cvWire, byte(v)))
}
