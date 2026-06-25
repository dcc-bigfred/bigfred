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
	if client.Paired == nil || s.cfg.Drive == nil {
		return
	}
	addr := client.CurrentLoco()
	if addr == 0 {
		if s.log != nil {
			s.log.WithField("client", client.Key).Info("z21 estop ignored: no active loco")
		}
		_ = s.writeUDP(remote, client.Key, buildBCStoppedReply())
		return
	}
	scope := remotes.DriveScope{
		AllowedAddrs:     client.Paired.AllowedAddrs,
		AllowAllVehicles: client.Paired.AllowAllVehicles,
	}
	if !s.cfg.Drive.AuthorizeDrive(client.Paired.UserID, addr, scope) {
		if s.log != nil {
			s.log.WithFields(logrus.Fields{
				"client": client.Key,
				"loco":   addr,
			}).Info("z21 estop rejected: not authorized")
		}
		return
	}
	session := remotes.HandsetSession{ClientKey: client.Key, UserID: client.Paired.UserID}
	s.cfg.Drive.ApplyHandsetPilotEStop(ctx, session, addr)
	if s.log != nil {
		s.log.WithFields(logrus.Fields{
			"client": client.Key,
			"userId": client.Paired.UserID,
			"loco":   addr,
		}).Info("z21 handset estop")
	}
	_ = s.writeUDP(remote, client.Key, buildBCStoppedReply())
}

func (s *Server) handleTrackPowerOff(ctx context.Context, client *Client) {
	if client.Paired == nil || s.cfg.Drive == nil {
		return
	}
	if err := s.cfg.Drive.TriggerLayoutRadioStop(ctx, client.Paired.UserID, contract.RemoteProtocolZ21); err != nil && s.log != nil {
		s.log.WithError(err).WithField("client", client.Key).Warn("z21 radio stop publish failed")
		return
	}
	if s.log != nil {
		s.log.WithFields(logrus.Fields{
			"client": client.Key,
			"userId": client.Paired.UserID,
		}).Info("z21 handset radio stop")
	}
}
