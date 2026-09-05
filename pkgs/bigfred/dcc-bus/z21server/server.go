package z21server

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotepairing"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotes"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/remotes/inbound"
	"github.com/dcc-bigfred/proto/go/pkgs/drive"
	z21proto "github.com/dcc-bigfred/proto/go/pkgs/z21"
)

// GatewayName is the remotes gateway factory key for Z21 LAN.
const GatewayName = contract.RemoteProtocolZ21

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

	OnListening func(ctx context.Context)

	Drive       remotes.InboundDrivePort
	Coordinator *remotes.Coordinator
	Store       *remotepairing.Store
	Log         *logrus.Logger
}

// Server listens for Z21 LAN UDP and answers handshake packets.
type Server struct {
	cfg         Config
	log         *logrus.Logger
	conn        *net.UDPConn
	registry    *Registry
	pairing     *PairingHandler
	adapter     *Adapter
	coordinator *remotes.Coordinator
	virtual     *remotes.VirtualLocoStore
	dispatch    *dispatcher

	runCtx atomic.Value // context.Context
	mu     sync.Mutex
	proto  *z21proto.Server
}

const (
	// dispatchShards spreads per-client packet processing so one slow
	// Redis round-trip cannot stall handsets on other shards.
	dispatchShards = 32
	// dispatchShardBuf per-shard queue depth. Large enough to absorb
	// bursts from one handset; when exceeded the read loop falls back
	// to inline (counted via dispatcher.InlineFallbacks) rather than
	// stalling.
	dispatchShardBuf = 128
	// sessionSyncStale is the safety window after which a per-packet
	// sync re-fetches the session from Redis. Event-driven sync (WS-1)
	// keeps the registry fresh in between; this only catches a lost
	// pub/sub event or daemon restart.
	sessionSyncStale = 30 * time.Second
)

// New validates cfg and returns a server that is not yet listening.
func New(cfg Config) (*Server, error) {
	if cfg.LayoutID == 0 || cfg.CommandStationID == 0 {
		return nil, errors.New("z21server: layout and command station id are required")
	}
	if cfg.Port == 0 && (cfg.Bind == "" || cfg.Bind == "0.0.0.0") {
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
	wire := NewWireState()
	var inboundReg *inbound.ClientRegistry
	if cfg.Coordinator != nil {
		inboundReg = cfg.Coordinator.Registry()
	}
	registry := NewRegistry(inboundReg, wire)
	s := &Server{
		cfg:         cfg,
		log:         log,
		registry:    registry,
		coordinator: cfg.Coordinator,
	}
	if cfg.Coordinator != nil {
		s.virtual = cfg.Coordinator.VirtualLocos()
		cfg.Coordinator.RegisterOnEvict(func(key string) {
			wire.Remove(key)
			s.disconnectProto(key)
		})
		cfg.Coordinator.RegisterSessionSyncHandler(contract.RemoteProtocolZ21, func(ctx context.Context, clientKey string) {
			s.syncPairedByKey(ctx, clientKey)
		})
	} else {
		s.virtual = remotes.NewVirtualLocoStore()
	}
	s.pairing = NewPairingHandler(cfg.Store, cfg.LayoutID, cfg.CommandStationID, s.registry, func(ctx context.Context) {
		if s.coordinator != nil {
			s.coordinator.PublishSnapshotThrottled(ctx)
		}
	}, func(ctx context.Context, evictedClientKey string) {
		if s.coordinator != nil {
			s.coordinator.Evict(ctx, evictedClientKey)
		} else {
			s.registry.Remove(evictedClientKey)
		}
	})
	if cfg.Drive != nil {
		s.adapter = NewAdapter(s, cfg.Drive)
	}
	return s, nil
}

// NewGateway builds a Z21 inbound listener from shared remotes wiring.
func NewGateway(_ context.Context, cfg remotes.GatewayConfig) (remotes.RemoteProtocol, error) {
	z21 := Config{
		LayoutID:         cfg.LayoutID,
		CommandStationID: cfg.CommandStationID,
		Drive:            cfg.Drive,
		Coordinator:      cfg.Coordinator,
		Store:            cfg.Store,
		Log:              cfg.Log,
	}
	if cfg.Extra != nil {
		if v, ok := cfg.Extra["bind"].(string); ok {
			z21.Bind = v
		}
		if v, ok := cfg.Extra["port"].(uint16); ok {
			z21.Port = v
		}
		if v, ok := cfg.Extra["ipStickiness"].(bool); ok {
			z21.IPStickiness = v
		}
		if v, ok := cfg.Extra["speedSteps"].(uint); ok {
			z21.SpeedSteps = v
		}
		if v, ok := cfg.Extra["onListening"].(func(context.Context)); ok {
			z21.OnListening = v
		}
	}
	return New(z21)
}

// Name implements remotes.RemoteProtocol.
func (s *Server) Name() string { return contract.RemoteProtocolZ21 }

// Run listens until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.runCtx.Store(ctx)
	bind := net.JoinHostPort(s.cfg.Bind, strconv.Itoa(int(s.cfg.Port)))
	ipStickiness := s.cfg.IPStickiness
	protoSrv, err := z21proto.Listen(bind, s,
		z21proto.WithSerial(s.cfg.Serial),
		z21proto.WithPeerTTL(0),
		z21proto.WithSystemStatePayload(s.effectiveSystemState().encode()),
		z21proto.WithClientKeyFunc(func(a *net.UDPAddr) drive.ClientID {
			return drive.ClientID(inbound.ClientKey(contract.RemoteProtocolZ21, inbound.EndpointFromAddr(a, ipStickiness)))
		}),
	)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.proto = protoSrv
	s.mu.Unlock()

	s.log.WithFields(logrus.Fields{
		"bind":             protoSrv.Addr().String(),
		"layoutId":         s.cfg.LayoutID,
		"commandStationId": s.cfg.CommandStationID,
		"ipStickiness":     s.cfg.IPStickiness,
	}).Info("z21 inbound server listening")

	if s.cfg.OnListening != nil {
		go s.cfg.OnListening(ctx)
	}

	if s.coordinator != nil {
		go func() { _ = s.coordinator.PublishSnapshot(ctx) }()
	}

	<-ctx.Done()
	s.mu.Lock()
	s.proto = nil
	s.mu.Unlock()
	return protoSrv.Close()
}

func (s *Server) protoServer() *z21proto.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.proto
}

func (s *Server) disconnectProto(key string) {
	if p := s.protoServer(); p != nil {
		p.Disconnect(drive.ClientID(key))
	}
}

func (s *Server) ctx() context.Context {
	if v := s.runCtx.Load(); v != nil {
		return v.(context.Context)
	}
	return context.Background()
}

func (s *Server) startDispatch() {
	if s.dispatch == nil {
		s.dispatch = newDispatcher(dispatchShards, dispatchShardBuf)
	}
}

func (s *Server) stopDispatch() {
	if s.dispatch != nil {
		s.dispatch.close()
		s.dispatch = nil
	}
}

func (s *Server) handlePacket(ctx context.Context, remote *net.UDPAddr, pkt []byte) {
	now := time.Now().UTC()
	client := s.registry.Touch(remote, now, s.cfg.IPStickiness)
	// Reconcile the session against Redis only on first sight (covers
	// daemon restart) or after the safety window — REST mutations arrive
	// via the session-sync pub/sub channel and update the registry out of
	// band, so the per-packet Redis GET is gone from the hot path.
	if s.registry.NeedsSync(client.Key, sessionSyncStale) {
		s.syncPaired(ctx, client)
		s.registry.MarkSynced(client.Key)
	}
	s.noteClientActivity(ctx, client)

	isPaired := s.registry.IsPaired(client.Key)
	if isPaired && s.cfg.Store != nil {
		// Stage the lastSeenAt update for the coordinator's batched
		// flusher (WS-1b) instead of a per-packet Redis SET.
		s.registry.MarkSeenDirty(client.Key, contract.NowMS())
	}

	_, header, ok := packetHeader(pkt)
	if !ok || !validFrame(pkt) {
		s.logUnhandled(client.Key, pkt, isPaired, "truncated")
		return
	}

	paired := isPaired
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
		s.handlePOMWrite(ctx, remote, client, pkt)
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

	if !isPaired {
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
		if isGetLocoInfo(header, pkt) {
			handled = true
			if addr, ok := parseGetLocoInfo(pkt); ok {
				s.sendVirtualLoco(ctx, client, s.virtual.Snapshot(client.Key, addr))
			}
			return
		}
		if isSetLocoDrive(header, pkt) {
			handled = true
			if addr, speed, forward, estop, ok := parseSetLocoDrive(pkt); ok {
				if estop {
					speed = 0
				}
				s.sendVirtualLoco(ctx, client, s.virtual.SetSpeed(client.Key, addr, speed, forward))
			}
			return
		}
		if isSetLocoEStop(header, pkt) {
			handled = true
			if addr, ok := parseSetLocoEStop(pkt); ok {
				s.sendVirtualLoco(ctx, client, s.virtual.SetSpeed(client.Key, addr, 0, true))
			}
			return
		}
		if isSetLocoFunction(header, pkt) {
			handled = true
			if addr, fn, sw, ok := parseSetLocoFunction(pkt); ok {
				var snap contract.LocoStateWire
				if sw == funcSwitchToggle {
					snap = s.virtual.ToggleFunction(client.Key, addr, fn)
				} else {
					snap = s.virtual.SetFunction(client.Key, addr, fn, sw == funcSwitchOn)
				}
				s.sendVirtualLoco(ctx, client, snap)
				if sw == funcSwitchOn || sw == funcSwitchToggle {
					s.handleUnpairedPairingFn(ctx, client, addr, fn)
				}
			}
			return
		}
		if isSetLocoFunctionGroup(header, pkt) {
			handled = true
			if addr, updates, ok := parseSetLocoFunctionGroup(pkt); ok && len(pkt) >= 9 {
				for _, u := range updates {
					s.virtual.SetFunction(client.Key, addr, u.fn, u.on)
				}
				s.sendVirtualLoco(ctx, client, s.virtual.Snapshot(client.Key, addr))
				for _, fn := range s.registry.PairingFnRisingEdges(client.Key, pkt[5], pkt[8]) {
					if s.handleUnpairedPairingFn(ctx, client, addr, fn) {
						break
					}
				}
			}
			return
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
	case isSetTrackPowerOn(header, pkt):
		handled = true
		s.handleTrackPowerOn(ctx, client)
	case isSetLocoEStop(header, pkt):
		handled = true
		s.adapter.HandleSetLocoEStop(ctx, client, pkt)
	case isPurgeLoco(header, pkt):
		handled = true
		s.adapter.HandlePurgeLoco(client, pkt)
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

func isSetLocoEStop(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 8 && pkt[4] == 0x92
}

func isPurgeLoco(header uint16, pkt []byte) bool {
	return header == HeaderXBus && len(pkt) >= 9 && pkt[4] == 0xE3 && pkt[5] == 0x44
}

func (s *Server) handleUnpairedPairingFn(ctx context.Context, client *Client, locoAddr uint16, fn int) bool {
	if s.pairing == nil {
		return false
	}
	s.registry.SubscribeLoco(client.Key, locoAddr)
	_, active := s.pairing.HandleFn(ctx, client, fn)
	if active == nil {
		return false
	}
	s.syncPaired(ctx, client)
	s.clearVirtualLoco(client.Key)
	fields := pairingLogFields(active)
	fields["client"] = client.Key
	fields["pairingCV3"] = active.PairingCV3
	fields["pairingCV4"] = active.PairingCV4
	s.log.WithFields(fields).Info("z21 handset paired via function keys")
	return true
}

func (s *Server) sendVirtualLoco(ctx context.Context, client *Client, snap contract.LocoStateWire) {
	_ = NewResponder(s, client).SendLocoState(ctx, snap)
}

func (s *Server) clearVirtualLoco(clientKey string) {
	if s.virtual != nil {
		s.virtual.RemoveClient(clientKey)
	}
}

func (s *Server) syncPaired(ctx context.Context, client *Client) {
	s.syncPairedByKey(ctx, client.Key)
}

// syncPairedByKey reconciles one client's paired state from Redis and
// clears the Z21 wire pairing buffer when the session is gone. Used both
// from the per-packet safety path and the session-sync pub/sub handler.
func (s *Server) syncPairedByKey(ctx context.Context, key string) {
	if s.cfg.Store == nil {
		s.registry.SetPaired(key, nil)
		return
	}
	active, ok, err := s.cfg.Store.GetActiveByClientKey(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, key)
	if err != nil || !ok {
		s.registry.SetPaired(key, nil)
		return
	}
	s.registry.SetPaired(key, &active)
}

func (s *Server) evictClient(ctx context.Context, key string) {
	if s.coordinator != nil {
		s.coordinator.Evict(ctx, key)
		return
	}
	s.registry.Remove(key)
	if s.cfg.Store != nil {
		if err := s.cfg.Store.Unpair(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, key); err != nil {
			s.log.WithError(err).WithField("client", key).Debug("z21 unpair on evict")
		}
	}
}

func (s *Server) writeUDP(addr *net.UDPAddr, clientKey string, pkt []byte) error {
	s.logTx(clientKey, pkt)
	if p := s.protoServer(); p != nil {
		p.SendTo(drive.ClientID(clientKey), pkt)
		return nil
	}
	if s.conn == nil {
		return errors.New("z21server: not listening")
	}
	_, err := s.conn.WriteToUDP(pkt, addr)
	return err
}

// RegistryForTest exposes the participant registry in tests.
func (s *Server) RegistryForTest() *Registry { return s.registry }

// PairingStoreForTest exposes the pairing store in tests.
func (s *Server) PairingStoreForTest() *remotepairing.Store { return s.cfg.Store }
