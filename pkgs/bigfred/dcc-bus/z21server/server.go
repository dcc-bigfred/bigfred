package z21server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

// Config wires the inbound Z21 UDP server for one dcc-bus daemon.
type Config struct {
	LayoutID         uint
	CommandStationID uint
	Bind             string
	Port             uint16
	Serial           uint32
	HwType           uint32
	FirmwareBCD      uint32
	SystemState      *SystemState
	SpeedSteps       uint

	IPStickiness bool

	Drive       remotes.InboundDrivePort
	Pairing      *z21pairing.Store
	ClientsPub   ClientsPublisher
	Log          *logrus.Logger
}

// Server listens for Z21 LAN UDP and answers handshake packets.
type Server struct {
	cfg      Config
	log      *logrus.Logger
	conn     *net.UDPConn
	registry *Registry
	pairing  *PairingHandler
	adapter  *Adapter

	clientsPub    ClientsPublisher
	clientsPubMu  sync.Mutex
	lastClientsPub time.Time
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
		cfg.Serial = rocoVirtualSerial(cfg.LayoutID, cfg.CommandStationID)
	}
	if cfg.SpeedSteps == 0 {
		cfg.SpeedSteps = 128
	}
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	s := &Server{
		cfg:        cfg,
		log:        log,
		registry:   NewRegistry(),
		clientsPub: cfg.ClientsPub,
	}
	s.pairing = NewPairingHandler(cfg.Pairing, cfg.LayoutID, cfg.CommandStationID, s.registry, func(ctx context.Context) {
		s.publishClientsSnapshotThrottled(ctx)
	})
	if cfg.Drive != nil {
		s.adapter = NewAdapter(s, cfg.Drive)
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
		"ipStickiness":     s.cfg.IPStickiness,
	}).Info("z21 inbound server listening")

	go s.runSweeper(ctx)
	go func() { _ = s.publishClientsSnapshot(ctx) }()

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
		datagram := append([]byte(nil), buf[:n]...)
		for _, pkt := range splitZ21Datagram(datagram) {
			s.handlePacket(ctx, remote, pkt)
		}
	}
}

func (s *Server) handlePacket(ctx context.Context, remote *net.UDPAddr, pkt []byte) {
	now := time.Now().UTC()
	client := s.registry.Touch(remote, now, s.cfg.IPStickiness)
	s.syncPaired(ctx, client)
	s.noteClientActivity(client)

	if client.Paired != nil && s.cfg.Pairing != nil {
		var sessionTTL time.Duration
		if s.cfg.IPStickiness {
			sessionTTL = StickySessionIdleEvictAfter * time.Second
		}
		_ = s.cfg.Pairing.TouchSeen(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, client.Key, contract.NowMS(), sessionTTL)
	}

	_, header, ok := packetHeader(pkt)
	if !ok {
		s.logUnhandled(client.Key, pkt, client.Paired != nil, "truncated")
		return
	}

	paired := client.Paired != nil
	handled := false
	defer func() {
		if !handled {
			s.logUnhandled(client.Key, pkt, paired, "no_handler")
		}
	}()

	s.logRx(client.Key, pkt, paired, "recv")

	if header == HeaderLogoff {
		handled = true
		s.evictClient(ctx, client.Key)
		return
	}

	if s.handleCommonLAN(ctx, remote, client, pkt) {
		handled = true
		return
	}

	if isPOMReadByte(pkt) {
		handled = true
		s.handlePOMRead(ctx, remote, client, pkt)
		return
	}

	if isPOMWriteByte(pkt) {
		handled = true
		s.handlePOMWrite(ctx, client, pkt)
		return
	}

	if isProgTrackCVRead(pkt) {
		handled = true
		s.handleProgTrackCVRead(ctx, remote, client, pkt)
		return
	}

	if isProgTrackCVWrite(pkt) {
		handled = true
		s.handleProgTrackCVWrite(ctx, remote, client, pkt)
		return
	}

	if client.Paired == nil {
		if reply, replied := s.handshakeReply(pkt); replied {
			handled = true
			_ = s.writeUDP(remote, client.Key, reply)
			return
		}
		if header == HeaderSetBroadcastFlags {
			handled = true
			s.applyBroadcastFlags(client, broadcastFlagsFromPkt(pkt))
			return
		}
		if isSetLocoFunction(header, pkt) {
			handled = true
			if addr, fn, on, ok := parseSetLocoFunction(pkt); ok && on {
				s.handleUnpairedPairingFn(ctx, client, addr, fn)
			}
			return
		}
		if isSetLocoFunctionGroup(header, pkt) {
			handled = true
			if addr, _, ok := parseSetLocoFunctionGroup(pkt); ok && len(pkt) >= 9 {
				for _, fn := range client.pairingFnRisingEdges(pkt[5], pkt[8]) {
					if s.handleUnpairedPairingFn(ctx, client, addr, fn) {
						break
					}
				}
			}
			return
		}
		if isDriveHeader(header, pkt) {
			handled = true
			s.log.WithField("client", client.Key).Info("z21 drive rejected: not paired")
		}
		return
	}

	if s.adapter == nil {
		return
	}

	switch {
	case isSetStop(header, pkt):
		handled = true
		s.handleSetStop(ctx, remote, client)
	case isSetTrackPowerOff(header, pkt):
		handled = true
		s.handleTrackPowerOff(ctx, client)
	case header == HeaderSetBroadcastFlags:
		handled = true
		s.adapter.HandleSetBroadcastFlags(client, pkt)
	case isSetLocoDrive(header, pkt):
		handled = true
		s.adapter.HandleSetLocoDrive(ctx, client, pkt)
	case isSetLocoFunction(header, pkt):
		handled = true
		s.adapter.HandleSetLocoFunction(ctx, client, pkt)
	case isSetLocoFunctionGroup(header, pkt):
		handled = true
		s.adapter.HandleSetLocoFunctionGroup(ctx, client, pkt)
	case isGetLocoInfo(header, pkt):
		handled = true
		s.adapter.HandleGetLocoInfo(ctx, client, pkt)
	default:
		if reply, replied := s.handshakeReply(pkt); replied {
			handled = true
			_ = s.writeUDP(remote, client.Key, reply)
			return
		}
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

func isSetLocoFunctionGroup(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 10 && pkt[4] == 0xE4 && pkt[5] >= 0x20 && pkt[5] <= 0x29
}

func isGetLocoInfo(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 6 && pkt[4] == 0xE3 && pkt[5] == 0xF0
}

func (s *Server) handleUnpairedPairingFn(ctx context.Context, client *Client, locoAddr uint16, fn int) bool {
	if s.pairing == nil {
		return false
	}
	client.SubscribeLoco(locoAddr)
	_, active := s.pairing.HandleFn(ctx, client, fn)
	if active == nil {
		return false
	}
	s.syncPaired(ctx, client)
	fields := pairingLogFields(active)
	fields["client"] = client.Key
	fields["pairingCV3"] = active.PairingCV3
	fields["pairingCV4"] = active.PairingCV4
	s.log.WithFields(fields).Info("z21 handset paired via function keys")
	return true
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

func (s *Server) runSweeper(ctx context.Context) {
	ticker := time.NewTicker(SweeperInterval * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepClients(ctx)
		}
	}
}

func (s *Server) sweepClients(ctx context.Context) {
	now := time.Now().UTC()
	for _, c := range s.registry.Snapshot() {
		idle := now.Sub(c.LastSeen)
		if c.Paired != nil {
			brakeAfter := time.Duration(contract.NormaliseHandsetBrakeSecs(c.Paired.HandsetBrakeSecs)) * time.Second
			if idle >= brakeAfter && !c.IdleBraked {
				s.brakeHandsetLocos(ctx, c)
				s.registry.SetIdleBraked(c.Key, true)
			}
		}
		evictAfter := time.Duration(IdleEvictAfter) * time.Second
		if s.cfg.IPStickiness && c.Paired != nil {
			evictAfter = StickySessionIdleEvictAfter * time.Second
		}
		if idle >= evictAfter {
			s.evictClient(ctx, c.Key)
		}
	}
	_ = s.publishClientsSnapshot(ctx)
}

func (s *Server) brakeHandsetLocos(ctx context.Context, client *Client) {
	if s.cfg.Drive == nil || client.Paired == nil {
		return
	}
	p := client.Paired
	scope := remotes.DriveScope{
		AllowedAddrs:     p.AllowedAddrs,
		AllowAllVehicles: p.AllowAllVehicles,
	}
	session := remotes.HandsetSession{ClientKey: client.Key, UserID: p.UserID}
	addrs := s.cfg.Drive.CollectHandsetDriveTargets(ctx, p.UserID, client.SubscribedLocos, scope)
	if len(addrs) == 0 {
		return
	}
	s.cfg.Drive.ApplyHandsetIdleBrake(ctx, session, client.SubscribedLocos, scope)
	s.log.WithFields(logrus.Fields{
		"client": client.Key,
		"userId": p.UserID,
		"addrs":  addrs,
	}).Info("z21 handset idle brake")
}

func (s *Server) evictClient(ctx context.Context, key string) {
	s.registry.Remove(key)
	if s.cfg.Pairing != nil {
		if err := s.cfg.Pairing.Unpair(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, key); err != nil {
			s.log.WithError(err).WithField("client", key).Debug("z21 unpair on evict")
		}
	}
	s.publishClientsSnapshotThrottled(ctx)
}

func (s *Server) writeUDP(addr *net.UDPAddr, clientKey string, pkt []byte) error {
	if s.conn == nil {
		return errors.New("z21server: not listening")
	}
	s.logTx(clientKey, pkt)
	_, err := s.conn.WriteToUDP(pkt, addr)
	return err
}

// RegistryForTest exposes the participant registry in tests.
func (s *Server) RegistryForTest() *Registry { return s.registry }

// PairingStoreForTest exposes the pairing store in tests.
func (s *Server) PairingStoreForTest() *z21pairing.Store { return s.cfg.Pairing }
