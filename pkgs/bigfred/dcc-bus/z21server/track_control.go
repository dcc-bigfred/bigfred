package z21server

import (
	"context"
	"net"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

func isSetStop(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 5 && pkt[4] == 0x80
}

func isSetTrackPowerOff(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 6 && pkt[4] == 0x21 && pkt[5] == 0x80
}

func buildBCStoppedReply() []byte {
	x := []byte{0x81}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func (s *Server) handleSetStop(ctx context.Context, remote *net.UDPAddr, client *Client) {
	p, ok := s.registry.Paired(client.Key)
	if !ok || s.cfg.Drive == nil {
		return
	}
	addr := s.registry.CurrentLoco(client.Key)
	if addr == 0 {
		if s.log != nil {
			s.log.WithField("client", client.Key).Info("z21 estop ignored: no active loco")
		}
		_ = s.writeUDP(remote, client.Key, buildBCStoppedReply())
		return
	}
	scope := remotes.DriveScope{
		AllowedAddrs:     p.AllowedAddrs,
		AllowAllVehicles: p.AllowAllVehicles,
	}
	if !s.cfg.Drive.AuthorizeDrive(p.UserID, addr, scope) {
		if s.log != nil {
			s.log.WithFields(logrus.Fields{
				"client": client.Key,
				"loco":   addr,
			}).Info("z21 estop rejected: not authorized")
		}
		_ = s.writeUDP(remote, client.Key, buildBCStoppedReply())
		return
	}
	session := remotes.HandsetSession{ClientKey: client.Key, UserID: p.UserID}
	s.cfg.Drive.ApplyHandsetPilotEStop(ctx, session, addr)
	if s.log != nil {
		s.log.WithFields(logrus.Fields{
			"client": client.Key,
			"userId": p.UserID,
			"loco":   addr,
		}).Info("z21 handset estop")
	}
	_ = s.writeUDP(remote, client.Key, buildBCStoppedReply())
}

func (s *Server) handleTrackPowerOff(ctx context.Context, client *Client) {
	p, ok := s.registry.Paired(client.Key)
	if !ok || s.cfg.Drive == nil {
		return
	}
	if err := s.cfg.Drive.TriggerLayoutRadioStop(ctx, p.UserID, contract.RemoteProtocolZ21); err != nil && s.log != nil {
		s.log.WithError(err).WithField("client", client.Key).Warn("z21 radio stop publish failed")
		return
	}
	if s.log != nil {
		s.log.WithFields(logrus.Fields{
			"client": client.Key,
			"userId": p.UserID,
		}).Info("z21 handset radio stop")
	}
}
