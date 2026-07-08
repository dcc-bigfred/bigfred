package withrottle

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/bigfred/remotes/inbound"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

// GatewayName is the remotes gateway factory key for WiThrottle TCP.
const GatewayName = contract.RemoteProtocolWithrottle

const (
	defaultPort           = contract.DefaultWithrottleInboundPort
	defaultSentinel       = contract.DefaultWithrottlePairingAddr
	defaultHeartbeatSecs  = contract.DefaultWithrottleHeartbeatSecs
	sessionSyncStale      = 30 * time.Second
	dispatchShards        = 32
	dispatchShardBuf      = 128
	maxLineLen            = 8192
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
	AllowedVehicles   contract.AllowedVehicles
	VehicleFunctions  contract.VehicleFunctions

	OnListening func(ctx context.Context)

	Drive       remotes.InboundDrivePort
	Coordinator *remotes.Coordinator
	Store       *remotepairing.Store
	Log         *logrus.Logger
}

// Server listens for WiThrottle TCP and dispatches line commands.
type Server struct {
	cfg         Config
	log         *logrus.Logger
	registry    *Registry
	pairing     *PairingHandler
	adapter     *Adapter
	coordinator *remotes.Coordinator
	virtual     *remotes.VirtualLocoStore
	dispatch    *dispatcher
	allowedMu   sync.RWMutex
	catalogueMu sync.RWMutex
	connWg      sync.WaitGroup
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
	if cfg.Port == 0 {
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
		cfg.Coordinator.RegisterOnEvict(wire.Remove)
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
	} else {
		s.virtual = remotes.NewVirtualLocoStore()
	}
	if cfg.Coordinator != nil {
		cfg.Coordinator.RegisterSessionSyncHandler(contract.RemoteProtocolWithrottle, func(ctx context.Context, clientKey string) {
			s.syncPairedByKey(ctx, clientKey)
		})
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

// Run listens until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", net.JoinHostPort(s.cfg.Bind, strconv.Itoa(int(s.cfg.Port))))
	if err != nil {
		return err
	}
	defer ln.Close()
	s.startDispatch()
	defer s.stopDispatch()

	s.log.WithFields(logrus.Fields{
		"bind":             ln.Addr().String(),
		"layoutId":         s.cfg.LayoutID,
		"commandStationId": s.cfg.CommandStationID,
	}).Info("withrottle inbound server listening")

	if s.cfg.OnListening != nil {
		go s.cfg.OnListening(ctx)
	}

	for {
		if ctx.Err() != nil {
			s.shutdownConnections()
			return nil
		}
		if tcpLn, ok := ln.(*net.TCPListener); ok {
			_ = tcpLn.SetDeadline(time.Now().Add(1 * time.Second))
		}
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				s.shutdownConnections()
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return err
		}
		go s.serveConn(ctx, conn)
	}
}

func (s *Server) shutdownConnections() {
	s.registry.wire.CloseAll()
	s.connWg.Wait()
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	s.connWg.Add(1)
	defer s.connWg.Done()
	r := bufio.NewReaderSize(conn, 512)
	defer conn.Close()
	readTimeout := s.readTimeout()
	var clientKey string
	for {
		if ctx.Err() != nil {
			s.handleDisconnect(ctx, clientKey, conn)
			return
		}
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			s.handleDisconnect(ctx, clientKey, conn)
			return
		}
		line, err := readLineLimited(r, maxLineLen)
		if err != nil {
			s.handleDisconnect(ctx, clientKey, conn)
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		if clientKey != "" {
			s.touchClientActivity(ctx, clientKey, line)
		}
		if clientKey == "" {
			if handled, key := s.handleAnonymous(ctx, conn, line); handled {
				if key != "" {
					clientKey = key
				}
				continue
			}
			continue
		}
		key := clientKey
		d := s.dispatch
		if d == nil {
			s.handleLine(ctx, conn, key, line)
			continue
		}
		d.dispatch(key, func() {
			s.handleLine(ctx, conn, key, line)
		})
	}
}

func (s *Server) readTimeout() time.Duration {
	secs := s.cfg.HeartbeatSecs
	if secs <= 0 {
		secs = defaultHeartbeatSecs
	}
	return time.Duration(secs*2+5) * time.Second
}

func (s *Server) handleAnonymous(ctx context.Context, conn net.Conn, line string) (handled bool, clientKey string) {
	switch {
	case strings.HasPrefix(line, "HU"):
		deviceID := strings.TrimSpace(line[2:])
		if deviceID == "" {
			return true, ""
		}
		clientKey := inbound.ClientKey(contract.RemoteProtocolWithrottle, deviceID)
		prevConn := s.registry.wire.Conn(clientKey)
		now := time.Now().UTC()
		client := s.registry.TouchByDeviceId(deviceID, conn, now)
		if prevConn != nil && prevConn != conn && s.registry.IsPaired(client.Key) {
			fields := logrus.Fields{
				"client":    client.Key,
				"deviceId":  deviceID,
				"newRemote": conn.RemoteAddr().String(),
			}
			if prevConn.RemoteAddr() != nil {
				fields["priorRemote"] = prevConn.RemoteAddr().String()
			}
			s.log.WithFields(fields).Warn("withrottle: paired handset device id taken over by new connection")
		}
		if s.registry.NeedsSync(client.Key, sessionSyncStale) {
			s.syncPaired(ctx, client)
			s.registry.MarkSynced(client.Key)
		}
		if s.cfg.Store != nil && s.registry.IsPaired(client.Key) {
			_ = s.cfg.Store.TouchSeen(ctx, s.cfg.LayoutID, s.cfg.CommandStationID, client.Key, contract.NowMS(), 0)
		}
		if !s.registry.initialBurstSent(client.Key) {
			s.sendInitialBurst(ctx, client.Key)
			s.registry.markInitialBurstSent(client.Key)
		}
		return true, client.Key
	case strings.HasPrefix(line, "N"):
		// N before HU is ignored.
		return true, ""
	case line == "*+" || line == "*-" || strings.HasPrefix(line, "*"):
		return true, ""
	default:
		return false, ""
	}
}

func (s *Server) handleLine(ctx context.Context, conn net.Conn, clientKey, line string) {
	s.registry.SetConn(clientKey, conn)
	client, ok := s.registry.Get(clientKey)
	if !ok {
		return
	}
	if s.registry.NeedsSync(clientKey, sessionSyncStale) {
		s.syncPaired(ctx, client)
		s.registry.MarkSynced(clientKey)
	}
	s.noteClientActivity(ctx, clientKey)

	paired := s.registry.IsPaired(clientKey)
	if paired && s.cfg.Store != nil {
		s.registry.MarkSeenDirty(clientKey, contract.NowMS())
	}

	switch {
	case strings.HasPrefix(line, "N"):
		name := strings.TrimSpace(line[1:])
		s.registry.setDeviceName(clientKey, name)
		if !paired {
			if consumed, active := s.pairing.HandleN(ctx, client, name); consumed && active != nil {
				fields := pairingLogFields(active)
				fields["client"] = clientKey
				fields["pairingCode"] = active.PairingCode
				s.log.WithFields(fields).Info("withrottle handset paired via device name")
				return
			}
		}
		if paired || s.registry.initialBurstSent(clientKey) {
			_ = s.writeLine(clientKey, fmt.Sprintf("*%g", s.cfg.HeartbeatSecs))
			return
		}
		s.sendInitialBurst(ctx, clientKey)
		s.registry.markInitialBurstSent(clientKey)
		return
	case line == "Q":
		s.evictClient(ctx, clientKey)
		return
	case line == "*+":
		s.registry.setHeartbeatMonitor(clientKey, true)
		return
	case line == "*-":
		s.registry.setHeartbeatMonitor(clientKey, false)
		return
	case line == "*":
		return
	case strings.HasPrefix(line, "PPA"):
		// track power from client ignored in v1
		return
	case strings.HasPrefix(line, "M"):
		s.handleM(ctx, client, line, paired)
	default:
		// ignore unknown lines per spec §16.3
	}
}

func (s *Server) handleM(ctx context.Context, client *Client, line string, paired bool) {
	cmd, ok := parseMAction(line)
	if !ok {
		return
	}
	switch cmd.Op {
	case MOpSteal:
		_ = s.writeLine(client.Key, "HMSteal not supported")
		return
	case MOpAdd:
		s.handleAcquire(ctx, client, cmd, paired)
	case MOpRemove:
		s.handleRelease(ctx, client, cmd, paired)
	case MOpAction:
		s.handleThrottleAction(ctx, client, cmd, paired)
	case MOpLabels:
		if !paired {
			return
		}
		s.handleFunctionLabels(client.Key, cmd)
	}
}

func (s *Server) handleAcquire(ctx context.Context, client *Client, cmd MCommand, paired bool) {
	addr, ok := parseAcquireAddr(cmd.LocoKey, cmd.Properties)
	if !ok {
		_ = s.writeLine(client.Key, "HMInvalid acquire address")
		return
	}
	if !paired {
		if !allowUnpairedAcquire(addr, s.cfg.PairingAddr, paired) {
			_ = s.writeLine(client.Key, "HMNot paired")
			return
		}
		key := locoKeyForAddr(addr)
		s.registry.withThrottle(client.Key, cmd.ThrottleID, func(tw *throttleWire) {
			tw.locos[addr] = key
			tw.lastLoco = addr
		})
		s.registry.setSentinelAcquired(client.Key, true, cmd.ThrottleID)
		for _, reply := range buildSentinelAcquireReply(cmd.ThrottleID, addr) {
			_ = s.writeLine(client.Key, reply)
		}
		return
	}
	if s.adapter == nil {
		return
	}
	s.adapter.HandleAcquire(ctx, client, cmd)
}

func (s *Server) handleRelease(ctx context.Context, client *Client, cmd MCommand, paired bool) {
	if !paired && s.registry.sentinelAcquired(client.Key) {
		addr, _, ok := parseLocoKey(cmd.LocoKey)
		if ok && isSentinelAddr(addr, s.cfg.PairingAddr) {
			s.registry.setSentinelAcquired(client.Key, false, 0)
			s.registry.withThrottle(client.Key, cmd.ThrottleID, func(tw *throttleWire) {
				delete(tw.locos, addr)
			})
			s.registry.ClearPairingBuffer(client.Key)
		}
		_ = s.writeLine(client.Key, buildReleaseLine(cmd.ThrottleID, cmd.LocoKey))
		return
	}
	if !paired {
		return
	}
	if s.adapter != nil {
		s.adapter.HandleRelease(ctx, client, cmd)
	}
}

func (s *Server) handleThrottleAction(ctx context.Context, client *Client, cmd MCommand, paired bool) {
	if len(cmd.Properties) == 0 {
		return
	}
	prop := cmd.Properties[0]
	if !paired {
		if s.registry.sentinelAcquired(client.Key) {
			addr := s.cfg.PairingAddr
			switch {
			case len(prop) >= 2 && prop[0] == 'V':
				if wireSpeed, estop, ok := parseSpeedValue(prop); ok {
					speed := uint8(0)
					if !estop {
						speed = dccSpeedFromWire(wireSpeed, s.cfg.SpeedSteps)
					}
					forward := s.virtual.Snapshot(client.Key, addr).Forward
					s.sendVirtualLoco(ctx, client, cmd.ThrottleID, s.virtual.SetSpeed(client.Key, addr, speed, forward))
				}
				return
			case len(prop) >= 2 && prop[0] == 'R':
				forward := prop[1] != '0'
				cur := s.virtual.Snapshot(client.Key, addr)
				s.sendVirtualLoco(ctx, client, cmd.ThrottleID, s.virtual.SetSpeed(client.Key, addr, cur.Speed, forward))
				return
			case len(prop) >= 2 && (prop[0] == 'F' || prop[0] == 'f'):
				if fn, on, force, ok := parseFunctionAction(prop); ok {
					s.sendVirtualLoco(ctx, client, cmd.ThrottleID, s.virtual.SetFunction(client.Key, addr, fn, on))
					rising := s.registry.PairingFnRisingEdge(client.Key, fn, on)
					if pairingFnAccept(on, force, rising) {
						if consumed, active := s.pairing.HandleFn(ctx, client, fn); consumed && active != nil {
							fields := pairingLogFields(active)
							fields["client"] = client.Key
							fields["pairingCode"] = active.PairingCode
							s.log.WithFields(fields).Info("withrottle handset paired via function keys")
						}
					}
				}
				return
			}
		}
		return
	}
	if s.adapter != nil {
		s.adapter.HandleAction(ctx, client, cmd)
	}
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
	s.sendInitialBurst(ctx, clientKey)
	if active != nil {
		_ = s.writeLine(clientKey, fmt.Sprintf("HmPaired as %d", active.UserID))
	}
	if s.coordinator != nil {
		s.coordinator.PublishSnapshotThrottled(ctx)
	}
}

func (s *Server) sendVirtualLoco(ctx context.Context, client *Client, throttleID byte, snap contract.LocoStateWire) {
	_ = NewResponder(s, client, throttleID).SendLocoState(ctx, snap)
}

func (s *Server) clearVirtualLoco(clientKey string) {
	if s.virtual != nil {
		s.virtual.RemoveClient(clientKey)
	}
}

func (s *Server) sendInitialBurst(ctx context.Context, clientKey string) {
	_ = ctx
	paired := s.registry.IsPaired(clientKey)
	sess, _ := s.registry.Session(clientKey)
	_ = s.writeLine(clientKey, "VN2.0")
	_ = s.writeLine(clientKey, fmt.Sprintf("*%g", s.cfg.HeartbeatSecs))
	_ = s.writeLine(clientKey, s.trackPowerLine())
	_ = s.writeLine(clientKey, BuildRosterLine(sess, s.allowedVehicles(), s.cfg.PairingAddr, paired))
	_ = s.writeLine(clientKey, "HTBigFred")
}

func (s *Server) trackPowerLine() string {
	if s.cfg.TrackPowerOn {
		return "PPA1"
	}
	return "PPA0"
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
	sess, ok := s.registry.Session(key)
	if !ok {
		return
	}
	_ = s.writeLine(key, BuildRosterLine(sess, s.allowedVehicles(), s.cfg.PairingAddr, true))
}

func (s *Server) handleFunctionLabels(clientKey string, cmd MCommand) {
	addr, _, ok := parseLocoKey(cmd.LocoKey)
	if !ok {
		return
	}
	line := buildFunctionLabelLine(cmd.ThrottleID, cmd.LocoKey, s.functionsForAddr(addr))
	if line == "" {
		return
	}
	_ = s.writeLine(clientKey, line)
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
	for _, addr := range contract.VehicleFunctionsChangedAddrs(prev, snap) {
		s.pushLabelsForAcquired(addr)
	}
}

func (s *Server) pushLabelsForAcquired(addr uint16) {
	if s.registry == nil {
		return
	}
	for _, key := range s.registry.Subscribers(addr) {
		throttleID, locoKey, ok := s.registry.findThrottleForAddr(key, addr)
		if !ok {
			continue
		}
		line := buildFunctionLabelLine(throttleID, locoKey, s.functionsForAddr(addr))
		if line == "" {
			continue
		}
		_ = s.writeLine(key, line)
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

// UpdateAllowedVehicles refreshes the layout roster used for RL emission.
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

func (s *Server) handleDisconnect(ctx context.Context, clientKey string, staleConn net.Conn) {
	if clientKey == "" {
		return
	}
	if s.registry.wire.Conn(clientKey) != staleConn {
		return
	}
	s.evictClient(ctx, clientKey)
}

func (s *Server) noteClientActivity(ctx context.Context, clientKey string) {
	if s.registry.IsPaired(clientKey) && s.registry.IdleBraked(clientKey) {
		s.registry.ClearIdleBraked(clientKey)
	}
	if s.coordinator != nil {
		s.coordinator.NoteActivity(ctx, clientKey)
	}
}

func (s *Server) touchClientActivity(ctx context.Context, clientKey, line string) {
	if line == "*" || strings.HasPrefix(line, "N") || strings.HasPrefix(line, "M") {
		s.registry.touchLastSeen(clientKey, time.Now().UTC())
		if s.registry.IsPaired(clientKey) && s.cfg.Store != nil {
			s.registry.MarkSeenDirty(clientKey, contract.NowMS())
		}
	}
}

func (s *Server) writeLine(key, line string) error {
	return s.registry.WriteLine(key, line)
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

// RegistryForTest exposes the participant registry in tests.
func (s *Server) RegistryForTest() *Registry { return s.registry }
