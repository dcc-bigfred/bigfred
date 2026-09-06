package withrottle

import (
	"context"
	"errors"
	"fmt"
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
	wtproto "github.com/dcc-bigfred/proto/go/pkgs/withrottle"
)

// GatewayName is the remotes gateway factory key for WiThrottle TCP.
const GatewayName = contract.RemoteProtocolWithrottle

const (
	defaultPort          = contract.DefaultWithrottleInboundPort
	defaultSentinel      = contract.DefaultWithrottlePairingAddr
	defaultHeartbeatSecs = contract.DefaultWithrottleHeartbeatSecs
	sessionSyncStale     = 30 * time.Second
)

// IdleEvictAfter is the idle window before evicting an unpaired WiThrottle client.
const IdleEvictAfter = 120

// Config wires the inbound WiThrottle TCP server for one dcc-bus daemon.
type Config struct {
	LayoutID         uint
	CommandStationID uint
	Bind             string
	Port             uint16
	PairingAddr      uint16
	HeartbeatSecs    float64
	SpeedSteps       uint
	TrackPowerOn     bool
	AllowedVehicles  contract.AllowedVehicles
	VehicleFunctions contract.VehicleFunctions

	OnListening func(ctx context.Context)

	Drive       remotes.InboundDrivePort
	Coordinator *remotes.Coordinator
	Store       *remotepairing.Store
	Log         *logrus.Logger
}

// Server listens for WiThrottle TCP via proto.Listen and applies BigFred policy.
type Server struct {
	cfg         Config
	log         *logrus.Logger
	registry    *Registry
	pairing     *PairingHandler
	adapter     *Adapter
	coordinator *remotes.Coordinator
	virtual     *remotes.VirtualLocoStore
	allowedMu   sync.RWMutex
	catalogueMu sync.RWMutex

	runCtx atomic.Value // context.Context
	quit   sync.Map     // client key → struct{} for Q vs TCP drop

	mu    sync.Mutex
	proto *wtproto.Server
}

// HeartbeatTimeout returns coordinator policy timeout with grace slack.
func HeartbeatTimeout(secs float64) time.Duration {
	if secs <= 0 {
		return 0
	}
	return time.Duration(secs+5) * time.Second
}

// New validates cfg and returns a server that is not yet listening.
func New(cfg Config) (*Server, error) {
	if cfg.LayoutID == 0 || cfg.CommandStationID == 0 {
		return nil, errors.New("withrottle: layout and command station id are required")
	}
	if cfg.Port == 0 && (cfg.Bind == "" || cfg.Bind == "0.0.0.0") {
		cfg.Port = defaultPort
	}
	if cfg.PairingAddr == 0 {
		cfg.PairingAddr = defaultSentinel
	}
	if cfg.PairingAddr > 10239 {
		return nil, errors.New("withrottle: pairing addr must be <= 10239 for L addressing")
	}
	if cfg.HeartbeatSecs == 0 {
		cfg.HeartbeatSecs = defaultHeartbeatSecs
	}
	if cfg.Bind == "" {
		cfg.Bind = "0.0.0.0"
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
		cfg.Coordinator.RegisterSessionSyncHandler(contract.RemoteProtocolWithrottle, func(ctx context.Context, clientKey string) {
			s.syncPairedByKey(ctx, clientKey)
		})
	} else {
		s.virtual = remotes.NewVirtualLocoStore()
	}
	s.pairing = NewPairingHandler(cfg.Store, cfg.LayoutID, cfg.CommandStationID, s.registry,
		func(ctx context.Context, key string, active *contract.RemoteSessionWire) {
			s.onPaired(ctx, key, active)
		},
		func(ctx context.Context, evictedClientKey string) {
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

// NewGateway builds a WiThrottle inbound listener from shared remotes wiring.
func NewGateway(_ context.Context, cfg remotes.GatewayConfig) (remotes.RemoteProtocol, error) {
	wt := Config{
		LayoutID:         cfg.LayoutID,
		CommandStationID: cfg.CommandStationID,
		Drive:            cfg.Drive,
		Coordinator:      cfg.Coordinator,
		Store:            cfg.Store,
		Log:              cfg.Log,
		TrackPowerOn:     true,
	}
	if cfg.Extra != nil {
		if v, ok := cfg.Extra["bind"].(string); ok {
			wt.Bind = v
		}
		if v, ok := cfg.Extra["port"].(uint16); ok {
			wt.Port = v
		}
		if v, ok := cfg.Extra["pairingAddr"].(uint16); ok {
			wt.PairingAddr = v
		}
		if v, ok := cfg.Extra["heartbeatSecs"].(float64); ok {
			wt.HeartbeatSecs = v
		}
		if v, ok := cfg.Extra["speedSteps"].(uint); ok {
			wt.SpeedSteps = v
		}
		if v, ok := cfg.Extra["allowedVehicles"].(contract.AllowedVehicles); ok {
			wt.AllowedVehicles = v
		}
		if v, ok := cfg.Extra["vehicleFunctions"].(contract.VehicleFunctions); ok {
			wt.VehicleFunctions = v
		}
		if v, ok := cfg.Extra["onListening"].(func(context.Context)); ok {
			wt.OnListening = v
		}
	}
	return New(wt)
}

// Name implements remotes.RemoteProtocol.
func (s *Server) Name() string { return contract.RemoteProtocolWithrottle }

// Addr is the bound TCP address after Run has started listening.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proto == nil {
		return nil
	}
	return s.proto.Addr()
}

// Run listens until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.runCtx.Store(ctx)
	bind := net.JoinHostPort(s.cfg.Bind, strconv.Itoa(int(s.cfg.Port)))
	hb := s.cfg.HeartbeatSecs
	if hb <= 0 {
		hb = defaultHeartbeatSecs
	}
	protoSrv, err := wtproto.Listen(bind, s,
		wtproto.WithHeartbeatSecs(hb),
		wtproto.WithDeadman(false),
		wtproto.WithReadTimeout(time.Duration(hb*2+5)*time.Second),
		wtproto.WithServerName("BigFred"),
		wtproto.WithTrackOn(s.cfg.TrackPowerOn),
		wtproto.WithRosterProvider(s),
		wtproto.WithLabelProvider(s),
		wtproto.WithErrorHandler(func(err error) {
			s.log.WithError(err).Warn("withrottle inbound listener error")
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
	}).Info("withrottle inbound server listening")

	if s.cfg.OnListening != nil {
		go s.cfg.OnListening(ctx)
	}

	<-ctx.Done()
	s.mu.Lock()
	s.proto = nil
	s.mu.Unlock()
	return protoSrv.Close()
}

func (s *Server) protoServer() *wtproto.Server {
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

func (s *Server) writeLine(key, line string) error {
	if p := s.protoServer(); p != nil {
		return p.SendTo(drive.ClientID(key), line)
	}
	return s.registry.WriteLine(key, line)
}

func (s *Server) onPaired(ctx context.Context, clientKey string, active *contract.RemoteSessionWire) {
	s.clearVirtualLoco(clientKey)
	if s.registry.sentinelAcquired(clientKey) {
		throttleID := s.registry.sentinelThrottleID(clientKey)
		for _, line := range buildSentinelReleaseLines(throttleID, s.cfg.PairingAddr) {
			_ = s.writeLine(clientKey, line)
		}
		s.registry.setSentinelAcquired(clientKey, false, 0)
	}
	if p := s.protoServer(); p != nil {
		p.ResendBurst(drive.ClientID(clientKey))
	}
	if active != nil {
		_ = s.writeLine(clientKey, fmt.Sprintf("HmPaired as %d", active.UserID))
	}
	if s.coordinator != nil {
		s.coordinator.PublishSnapshotThrottled(ctx)
	}
}

func (s *Server) clearVirtualLoco(clientKey string) {
	if s.virtual != nil {
		s.virtual.RemoveClient(clientKey)
	}
}

func (s *Server) syncPaired(ctx context.Context, client *Client) {
	s.syncPairedByKey(ctx, client.Key)
}

func (s *Server) syncPairedByKey(ctx context.Context, key string) {
	if s.cfg.Store == nil {
		s.registry.SetPaired(key, nil)
		return
	}
	active, ok, err := s.cfg.Store.GetActiveByClientKey(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, key)
	if err != nil || !ok {
		s.registry.SetPaired(key, nil)
		prev, had := s.registry.Session(key)
		if had && prev != nil {
			s.emitRosterUpdate(key)
		}
		return
	}
	prev, had := s.registry.Session(key)
	s.registry.SetPaired(key, &active)
	if had && prev != nil && scopeChanged(prev, &active) {
		s.emitRosterUpdate(key)
	}
}

func scopeChanged(a, b *contract.RemoteSessionWire) bool {
	if a == nil || b == nil {
		return true
	}
	if a.AllowAllVehicles != b.AllowAllVehicles {
		return true
	}
	if len(a.AllowedAddrs) != len(b.AllowedAddrs) {
		return true
	}
	seen := make(map[uint16]struct{}, len(a.AllowedAddrs))
	for _, addr := range a.AllowedAddrs {
		seen[addr] = struct{}{}
	}
	for _, addr := range b.AllowedAddrs {
		if _, ok := seen[addr]; !ok {
			return true
		}
	}
	return false
}

func (s *Server) emitRosterUpdate(key string) {
	if p := s.protoServer(); p != nil {
		p.SendRoster(drive.ClientID(key))
	}
}

func (s *Server) functionsForAddr(addr uint16) []contract.FunctionDefinition {
	s.catalogueMu.RLock()
	snap := s.cfg.VehicleFunctions
	s.catalogueMu.RUnlock()
	for _, v := range snap.Vehicles {
		if v.Addr == addr {
			return v.Functions
		}
	}
	return nil
}

func (s *Server) allowedVehicles() contract.AllowedVehicles {
	s.allowedMu.RLock()
	defer s.allowedMu.RUnlock()
	return s.cfg.AllowedVehicles
}

// UpdateVehicleFunctions refreshes the layout function catalogue for acquire
// replies and M…L label requests.
func (s *Server) UpdateVehicleFunctions(snap contract.VehicleFunctions) {
	if s == nil {
		return
	}
	s.catalogueMu.Lock()
	prev := s.cfg.VehicleFunctions
	s.cfg.VehicleFunctions = snap
	s.catalogueMu.Unlock()
	p := s.protoServer()
	if p == nil {
		return
	}
	for _, addr := range contract.VehicleFunctionsChangedAddrs(prev, snap) {
		labels := functionLabels(s.functionsForAddr(addr))
		key := locoKeyForAddr(addr)
		for _, holder := range p.HoldersOf(addr) {
			if !s.registry.IsPaired(string(holder)) {
				continue
			}
			tid, locoKey, ok := s.registry.findThrottleForAddr(string(holder), addr)
			if !ok {
				tid, locoKey = '0', key
			}
			line := wtproto.FormatLabelLine(tid, locoKey, labels)
			if line == "" {
				continue
			}
			_ = p.SendTo(holder, line)
		}
	}
}

func (s *Server) UpdateAllowedVehicles(snap contract.AllowedVehicles) {
	if s == nil {
		return
	}
	s.allowedMu.Lock()
	s.cfg.AllowedVehicles = snap
	s.allowedMu.Unlock()
	if s.registry == nil {
		return
	}
	for _, client := range s.registry.Snapshot() {
		if client.Session == nil {
			continue
		}
		s.emitRosterUpdate(client.Key)
	}
}

func (s *Server) evictClient(ctx context.Context, key string) {
	if s.coordinator != nil {
		s.coordinator.Evict(ctx, key)
		return
	}
	s.registry.Remove(key)
	if s.cfg.Store != nil {
		if err := s.cfg.Store.Unpair(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, key); err != nil {
			s.log.WithError(err).WithField("client", key).Debug("withrottle unpair on evict")
		}
	}
}

func (s *Server) dropPresence(ctx context.Context, key string) {
	if s.coordinator != nil {
		s.coordinator.DropPresence(ctx, key)
		return
	}
	s.registry.Remove(key)
}

func (s *Server) noteClientActivity(ctx context.Context, clientKey string) {
	if s.registry.IsPaired(clientKey) && s.registry.IdleBraked(clientKey) {
		s.registry.ClearIdleBraked(clientKey)
	}
	if s.coordinator != nil {
		s.coordinator.NoteActivity(ctx, clientKey)
	}
}

// OnLocoStateChanged pushes M…A lines to paired holders except the commander.
func (s *Server) OnLocoStateChanged(ctx context.Context, snap contract.LocoStateWire, originClientKey string) {
	_ = ctx
	p := s.protoServer()
	if p == nil || s.registry == nil {
		return
	}
	for _, holder := range p.HoldersOf(snap.Address) {
		key := string(holder)
		if key == originClientKey {
			continue
		}
		client, ok := s.registry.Get(key)
		if !ok || client.Session == nil {
			continue
		}
		throttleID, locoKey, ok := s.registry.findThrottleForAddr(key, snap.Address)
		if !ok {
			locoKey = locoKeyForAddr(snap.Address)
			throttleID = '0'
		}
		s.registry.setLastSpeed(key, throttleID, snap.Address, snap.Speed)
		for _, line := range buildLocoNotify(throttleID, locoKey, snap, s.cfg.SpeedSteps) {
			_ = p.SendTo(holder, line)
		}
	}
}

func buildLocoNotify(throttleID byte, locoKey string, snap contract.LocoStateWire, speedSteps uint) []string {
	id := string(throttleID)
	speed := wireSpeedFromDCC(snap.Speed, speedSteps)
	dir := 0
	if snap.Forward {
		dir = 1
	}
	lines := []string{
		fmt.Sprintf("M%sA%s%sV%d", id, locoKey, propSep, speed),
		fmt.Sprintf("M%sA%s%sR%d", id, locoKey, propSep, dir),
	}
	if len(snap.Functions) > 0 {
		fns := make([]int, 0, len(snap.Functions))
		for fn := range snap.Functions {
			if fn > maxWiThrottleFunction {
				continue
			}
			fns = append(fns, fn)
		}
		sortInts(fns)
		for _, fn := range fns {
			state := 0
			if snap.Functions[fn] {
				state = 1
			}
			lines = append(lines, fmt.Sprintf("M%sA%s%sF%d%d", id, locoKey, propSep, state, fn))
		}
	}
	return lines
}

func sortInts(fns []int) {
	for i := 1; i < len(fns); i++ {
		for j := i; j > 0 && fns[j] < fns[j-1]; j-- {
			fns[j], fns[j-1] = fns[j-1], fns[j]
		}
	}
}

func functionLabels(defs []contract.FunctionDefinition) []string {
	if len(defs) == 0 {
		return nil
	}
	maxFn := 0
	labels := make([]string, maxWiThrottleFunction+1)
	for _, d := range defs {
		n := int(d.Num)
		if n < 0 || n > maxWiThrottleFunction {
			continue
		}
		if n > maxFn {
			maxFn = n
		}
		labels[n] = d.FunctionLabel()
	}
	return labels[:maxFn+1]
}

// RegistryForTest exposes the participant registry in tests.
func (s *Server) RegistryForTest() *Registry { return s.registry }

var _ remotes.LocoStateObserver = (*Server)(nil)
var _ remotes.RemoteProtocol = (*Server)(nil)
