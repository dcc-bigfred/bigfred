package z21server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

// Config wires the inbound Z21 UDP server for one dcc-bus daemon.
type Config struct {
	LayoutID         uint
	CommandStationID uint
	Bind             string
	Port             uint16
	Serial           uint32
	Pairing          *z21pairing.Store
	Log              *logrus.Logger
}

// Server listens for Z21 LAN UDP and answers handshake packets.
type Server struct {
	cfg      Config
	log      *logrus.Logger
	conn     *net.UDPConn
	registry *Registry
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
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	return &Server{
		cfg:      cfg,
		log:      log,
		registry: NewRegistry(),
	}, nil
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

	_, header, ok := packetHeader(pkt)
	if !ok {
		return
	}

	if header == HeaderLogoff {
		s.evictClient(ctx, client.Key)
		return
	}

	if reply, handled := s.handshakeReply(header); handled {
		_ = s.writeUDP(remote, reply)
		return
	}

	if isDriveHeader(header, pkt) {
		s.log.WithField("client", client.Key).Debug("z21 drive rejected: not paired")
		return
	}

	if !isHandshakeHeader(header) && header != HeaderSetBroadcastFlags {
		s.log.WithFields(logrus.Fields{
			"client": client.Key,
			"header": fmt.Sprintf("0x%04X", header),
		}).Debug("z21 unhandled packet while unpaired")
	}
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
