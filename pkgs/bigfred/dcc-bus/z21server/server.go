package z21server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/cmd"
	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

// Config wires the inbound Z21 UDP server for one dcc-bus daemon.
type Config struct {
	LayoutID         uint
	CommandStationID uint
	Bind             string
	Port             uint16
	Serial           uint32
	SpeedSteps       uint

	Router  *cmd.Router
	Pairing *z21pairing.Store
	Log     *logrus.Logger
}

// Server listens for Z21 LAN UDP and answers handshake packets.
type Server struct {
	cfg      Config
	log      *logrus.Logger
	conn     *net.UDPConn
	registry *Registry
	pairing  *PairingHandler
	adapter  *Adapter
}

// New validates cfg and returns a server that is not yet listening.
func New(cfg Config) (*Server, error) {
	if cfg.LayoutID == 0 || cfg.CommandStationID == 0 {
		return nil, errors.New("z21server: layout and command station id are required")
	}
	if cfg.Port == 0 {
		cfg.Port = 21105
	}
	if cfg.Bind == "" {
		cfg.Bind = "0.0.0.0"
	}
	if cfg.Serial == 0 {
		cfg.Serial = virtualSerial(cfg.LayoutID, cfg.CommandStationID)
	}
	if cfg.SpeedSteps == 0 {
		cfg.SpeedSteps = 128
	}
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	s := &Server{
		cfg:      cfg,
		log:      log,
		registry: NewRegistry(),
	}
	s.pairing = NewPairingHandler(cfg.Pairing, cfg.LayoutID, cfg.CommandStationID, s.registry)
	if cfg.Router != nil {
		s.adapter = NewAdapter(s, cfg.Router)
	}
	return s, nil
}

// Run listens until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(s.cfg.Bind, fmt.Sprintf("%d", s.cfg.Port)))
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	s.conn = conn
	defer conn.Close()

	s.log.WithFields(logrus.Fields{
		"bind":             conn.LocalAddr().String(),
		"layoutId":         s.cfg.LayoutID,
		"commandStationId": s.cfg.CommandStationID,
	}).Info("z21 inbound server listening")

	go s.runSweeper(ctx)

	buf := make([]byte, 2048)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return err
		}
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return err
		}
		pkt := append([]byte(nil), buf[:n]...)
		s.handlePacket(ctx, remote, pkt)
	}
}

func (s *Server) handlePacket(ctx context.Context, remote *net.UDPAddr, pkt []byte) {
	now := time.Now().UTC()
	client := s.registry.Touch(remote, now)
	s.syncPaired(ctx, client)

	if client.Paired != nil && s.cfg.Pairing != nil {
		_ = s.cfg.Pairing.TouchSeen(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, client.Key, contract.NowMS())
	}

	_, header, ok := packetHeader(pkt)
	if !ok {
		return
	}

	if header == HeaderLogoff {
		s.evictClient(ctx, client.Key)
		return
	}

	if isPairingPOM(pkt) {
		cvWire, value, ok := parsePOMWriteByte(pkt)
		if ok {
			if _, active := s.pairing.Handle(ctx, client, cvWire, value); active != nil {
				s.syncPaired(ctx, client)
				s.log.WithFields(logrus.Fields{
					"client": client.Key,
					"userId": active.UserID,
				}).Info("z21 handset paired via CV3/CV4")
			}
		}
		return
	}

	if client.Paired == nil {
		if reply, handled := s.handshakeReply(header); handled {
			_ = s.writeUDP(remote, reply)
			return
		}
		if header == HeaderSetBroadcastFlags {
			client.BroadcastFlags = broadcastFlagsFromPkt(pkt)
			return
		}
		if isDriveHeader(header, pkt) {
			s.log.WithField("client", client.Key).Debug("z21 drive rejected: not paired")
		}
		return
	}

	if s.adapter == nil {
		return
	}

	switch {
	case header == HeaderSetBroadcastFlags:
		s.adapter.HandleSetBroadcastFlags(client, pkt)
	case isSetLocoDrive(header, pkt):
		s.adapter.HandleSetLocoDrive(ctx, client, pkt)
	case isSetLocoFunction(header, pkt):
		s.adapter.HandleSetLocoFunction(ctx, client, pkt)
	case isGetLocoInfo(header, pkt):
		s.adapter.HandleGetLocoInfo(ctx, client, pkt)
	default:
		if reply, handled := s.handshakeReply(header); handled {
			_ = s.writeUDP(remote, reply)
			return
		}
		s.log.WithFields(logrus.Fields{
			"client": client.Key,
			"header": fmt.Sprintf("0x%04X", header),
		}).Debug("z21 unhandled packet from paired client")
	}
}

func broadcastFlagsFromPkt(pkt []byte) uint32 {
	if len(pkt) < 8 {
		return 0
	}
	return uint32(pkt[4]) | uint32(pkt[5])<<8 | uint32(pkt[6])<<16 | uint32(pkt[7])<<24
}

func isSetLocoDrive(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 6 && pkt[4] == 0xE4 && (pkt[5]&0xF0) == 0x10
}

func isSetLocoFunction(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 6 && pkt[4] == 0xE4 && pkt[5] == 0xF8
}

func isGetLocoInfo(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 6 && pkt[4] == 0xE3 && pkt[5] == 0xF0
}

func (s *Server) syncPaired(ctx context.Context, client *Client) {
	if s.cfg.Pairing == nil {
		client.Paired = nil
		return
	}
	active, ok, err := s.cfg.Pairing.GetActiveByClientKey(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, client.Key)
	if err != nil || !ok {
		client.Paired = nil
		return
	}
	copy := active
	client.Paired = &copy
}

func (s *Server) handshakeReply(header uint16) ([]byte, bool) {
	switch header {
	case HeaderGetSerialNumber:
		return buildSerialReply(s.cfg.Serial), true
	case HeaderGetHWInfo:
		return buildHWInfoReply(HwTypeZ21Small, FirmwareVersion12), true
	case HeaderSystemStateGetData:
		return buildSystemStateReply(), true
	case HeaderGetBroadcastFlags:
		data := make([]byte, 4)
		return buildReply(HeaderGetBroadcastFlags, data), true
	default:
		return nil, false
	}
}

func (s *Server) runSweeper(ctx context.Context) {
	ticker := time.NewTicker(SweeperInterval * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().UTC().Add(-IdleEvictAfter * time.Second)
			for _, key := range s.registry.EvictIdle(cutoff) {
				s.evictClient(ctx, key)
			}
		}
	}
}

func (s *Server) evictClient(ctx context.Context, key string) {
	s.registry.Remove(key)
	if s.cfg.Pairing != nil {
		if err := s.cfg.Pairing.Unpair(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, key); err != nil {
			s.log.WithError(err).WithField("client", key).Debug("z21 unpair on evict")
		}
	}
}

func (s *Server) writeUDP(addr *net.UDPAddr, pkt []byte) error {
	if s.conn == nil {
		return errors.New("z21server: not listening")
	}
	_, err := s.conn.WriteToUDP(pkt, addr)
	return err
}

func virtualSerial(layoutID, commandStationID uint) uint32 {
	return uint32(0xBF210000) | uint32(layoutID<<8) | uint32(commandStationID&0xFF)
}

// RegistryForTest exposes the participant registry in tests.
func (s *Server) RegistryForTest() *Registry { return s.registry }

// PairingStoreForTest exposes the pairing store in tests.
func (s *Server) PairingStoreForTest() *z21pairing.Store { return s.cfg.Pairing }
