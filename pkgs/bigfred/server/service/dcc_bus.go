package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	dccbuscli "github.com/keskad/loco/pkgs/bigfred/dcc-bus/cli"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
	"github.com/keskad/loco/pkgs/bigfred/server/microinit"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

// DccBusGroupName is retained for compatibility. dcc-bus services are
// persisted in the BigFred microinit drop-in group.
const DccBusGroupName = GroupDccBus

// ErrNoDccBusPortsAvailable is returned by EnsureRunning when the
// configured port pool is exhausted.
var ErrNoDccBusPortsAvailable = svcerrors.ErrNoDCCBusPortsAvailable

// ErrDccBusUnavailable is returned when EnsureRunning could not
// confirm the daemon reached RUNNING + dial-able within the timeout.
// Surface this to the WS layer as `dcc_bus_unavailable`.
var ErrDccBusUnavailable = errors.New("dcc-bus daemon unavailable")

var ErrServiceManagerNotWired = errors.New("dcc-bus: service manager is not wired")

// Deprecated: use ErrServiceManagerNotWired.
var ErrServicesNotWired = ErrServiceManagerNotWired

// DccBusConfig configures DccBusService. Defaults match §7e.2.
type DccBusConfig struct {
	// Executable is the absolute path of the loco-server binary used
	// to launch `<executable> dcc-bus …`. Defaults to os.Args[0].
	Executable string
	// RedisAddr is forwarded as the daemon's --redis-addr flag.
	RedisAddr string
	// JWTSecret is forwarded verbatim via --jwt-secret. The
	// microinit render path applies shell quoting so spaces /
	// quotes survive.
	JWTSecret []byte
	// PortMin / PortMax bracket the TCP port pool. Default
	// 9200..9209 — see §7e.2.
	PortMin uint16
	PortMax uint16
	// SpawnTimeout is the budget EnsureRunning waits for a freshly-
	// spawned daemon to accept a WS dial. Default 10s — matches
	// microinit's startsecs + a small slack window.
	SpawnTimeout time.Duration
	// AllowedOrigins is forwarded as --allowed-origin flags so a
	// dev frontend can dial the daemon directly (production proxies
	// through loco-server and the slice is empty).
	AllowedOrigins []string
	// ProxyEnabled controls whether session.opened reports the
	// reverse-proxy path (default true) or the raw daemon port.
	ProxyEnabled bool
	// EnableTelemetry forwards --enable-telemetry to spawned dcc-bus
	// daemons when OTLPEndpoint is set.
	EnableTelemetry bool
	// OTLPEndpoint is the OTLP/gRPC address dcc-bus exports metrics to
	// (typically the local Alloy receiver).
	OTLPEndpoint string
	// ManagedRedis is true when microinit runs the redis service; dcc-bus
	// programs then declare dependsOn: ["redis"].
	ManagedRedis bool
}

// DccBusService is the loco-server-side orchestrator for dcc-bus
// daemons. It owns the port pool, drives microinit via
// SupervisordService, and exposes typed helpers to publish commands
// onto the daemon's Redis channels (§7e.6).
type DccBusService struct {
	cfg     DccBusConfig
	mgr     ServiceManager
	redis   *RedisService
	cs      *repo.CommandStations
	layouts *repo.Layouts
	log     *logrus.Logger

	mu      sync.Mutex
	ports   map[portKey]uint16 // (layoutID, commandStationID) -> port
	metrics *metrics.Metrics
}

type portKey struct {
	LayoutID         uint
	CommandStationID uint
}

// NewDccBusService returns a service ready to spawn daemons. The
// caller MUST run SupervisordService.Start before any EnsureRunning
// call. `redis` may be nil in tests that don't need the persistent
// port assignment; production wires it.
func NewDccBusService(cfg DccBusConfig, mgr ServiceManager, redis *RedisService, cs *repo.CommandStations, layouts *repo.Layouts, log *logrus.Logger) *DccBusService {
	if log == nil {
		log = logrus.New()
	}
	if cfg.Executable == "" {
		cfg.Executable = os.Args[0]
	}
	if cfg.PortMin == 0 {
		cfg.PortMin = 9200
	}
	if cfg.PortMax == 0 {
		cfg.PortMax = 9209
	}
	if cfg.SpawnTimeout == 0 {
		cfg.SpawnTimeout = 10 * time.Second
	}
	return &DccBusService{
		cfg:     cfg,
		mgr:     mgr,
		redis:   redis,
		cs:      cs,
		layouts: layouts,
		log:     log,
		ports:   make(map[portKey]uint16, 8),
	}
}

// HydratePorts loads previously-allocated port mappings from Redis
// so a loco-server restart preserves daemon ↔ port pairings. Call
// once during bootstrap, after SupervisordService.Start. Safe to
// call when Redis is empty.
func (d *DccBusService) HydratePorts(ctx context.Context) error {
	if d.redis == nil {
		return nil
	}
	rows, err := d.redis.Client().HGetAll(ctx, contract.DccBusPortsKey).Result()
	if err != nil {
		return fmt.Errorf("hgetall %s: %w", contract.DccBusPortsKey, err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, v := range rows {
		var layoutID, commandStationID uint
		if _, err := fmt.Sscanf(k, contract.DccBusPortsFieldTmpl, &layoutID, &commandStationID); err != nil {
			continue
		}
		p, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			continue
		}
		d.ports[portKey{LayoutID: layoutID, CommandStationID: commandStationID}] = uint16(p)
	}
	return nil
}

// LayoutIDsWithProgramForCS returns layout ids that currently have a
// port assignment for commandStationID. Used when catalogue rows change
// so microinit can restart daemons that are still running after the
// operator disconnects from the control plane.
func (d *DccBusService) LayoutIDsWithProgramForCS(commandStationID uint) []uint {
	d.mu.Lock()
	defer d.mu.Unlock()
	seen := make(map[uint]struct{})
	out := make([]uint, 0)
	for k := range d.ports {
		if k.CommandStationID != commandStationID {
			continue
		}
		if _, ok := seen[k.LayoutID]; ok {
			continue
		}
		seen[k.LayoutID] = struct{}{}
		out = append(out, k.LayoutID)
	}
	return out
}

// SetMetrics wires optional OpenTelemetry recorders for orchestration paths.
func (d *DccBusService) SetMetrics(m *metrics.Metrics) {
	d.metrics = m
}

// AllocatedPortCount returns how many dcc-bus daemons have a port assignment.
// Implements metrics.DccBusStatsReader.
func (d *DccBusService) AllocatedPortCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.ports)
}

// PortFor returns the TCP port assigned to (layoutID, commandStationID)
// or 0 when none has been allocated yet. Used by the reverse proxy
// to resolve `csId → port` without serializing on the daemon's mutex.
func (d *DccBusService) PortFor(layoutID, commandStationID uint) uint16 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ports[portKey{LayoutID: layoutID, CommandStationID: commandStationID}]
}

// programName returns the microinit program name for a daemon.
// The name MUST satisfy `[a-z][a-z0-9_-]*` to pass DesiredState
// validation, so we always lower-case + dash-join the numbers.
func programName(layoutID, commandStationID uint) string {
	return fmt.Sprintf("dcc-bus-%d-%d", layoutID, commandStationID)
}

// DccBusProgramStatus is one dcc-bus microinit program for a
// (layout, commandStation) pair, as shown in the admin popup and on
// the Unix control socket. WiThrottle/Z21 fields come from the
// command-station catalogue (the same values passed as
// --withrottle-port / --z21-port).
type DccBusProgramStatus struct {
	LayoutID          uint   `json:"layoutId"`
	LayoutName        string `json:"layoutName"`
	Name              string `json:"name"`
	Status            string `json:"status"`
	PID               int    `json:"pid,omitempty"`
	Running           bool   `json:"running"`
	CommandStationID  uint   `json:"commandStationId"`
	WithrottleEnabled bool   `json:"withrottleEnabled"`
	WithrottlePort    uint16 `json:"withrottlePort"`
	Z21Enabled        bool   `json:"z21Enabled"`
	Z21Port           uint16 `json:"z21Port"`
}

// ProgramsForCommandStation lists every dcc-bus microinit program
// for commandStationID across all layouts that have a port assignment
// or a microinit row. Absent programs are reported as STOPPED.
// When microinit is not wired (--no-supervisor), returns ErrServicesNotWired.
func (d *DccBusService) ProgramsForCommandStation(ctx context.Context, commandStationID uint) ([]DccBusProgramStatus, error) {
	if d.mgr == nil {
		return nil, ErrServiceManagerNotWired
	}
	rows, err := d.mgr.Status(ctx)
	if err != nil {
		return nil, err
	}

	candidates := make(map[uint]struct{})
	for _, id := range d.LayoutIDsWithProgramForCS(commandStationID) {
		candidates[id] = struct{}{}
	}
	statusByLayout := make(map[uint]ServiceState)
	for _, row := range rows {
		bare := row.Name
		if i := strings.LastIndex(bare, ":"); i >= 0 {
			bare = bare[i+1:]
		}
		layoutID, ok := parseDccBusProgramLayoutID(bare, commandStationID)
		if !ok {
			continue
		}
		candidates[layoutID] = struct{}{}
		st := row
		st.Name = programName(layoutID, commandStationID)
		statusByLayout[layoutID] = st
	}

	witEn, witPort, z21En, z21Port := d.catalogInboundPorts(ctx, commandStationID)

	out := make([]DccBusProgramStatus, 0, len(candidates))
	for layoutID := range candidates {
		name := programName(layoutID, commandStationID)
		st, found := statusByLayout[layoutID]
		if !found {
			st = ServiceState{Name: name, State: "stopped"}
		}
		layoutName := ""
		if d.layouts != nil {
			if layout, err := d.layouts.FindByID(ctx, layoutID); err == nil {
				layoutName = layout.Name
			}
		}
		out = append(out, DccBusProgramStatus{
			LayoutID:          layoutID,
			LayoutName:        layoutName,
			Name:              st.Name,
			Status:            st.State,
			PID:               st.PID,
			Running:           st.State == "running",
			CommandStationID:  commandStationID,
			WithrottleEnabled: witEn,
			WithrottlePort:    witPort,
			Z21Enabled:        z21En,
			Z21Port:           z21Port,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LayoutID < out[j].LayoutID })
	if d.log != nil {
		d.log.WithFields(logrus.Fields{
			"commandStationId": commandStationID,
			"count":            len(out),
		}).Debug("dcc-bus programs for command station")
	}
	return out, nil
}

// catalogInboundPorts returns WiThrottle/Z21 listen settings from the
// command-station row. Missing catalogue entries leave zeros/false so
// advertisers skip the service rather than guessing 12090/21105.
func (d *DccBusService) catalogInboundPorts(ctx context.Context, commandStationID uint) (witEn bool, witPort uint16, z21En bool, z21Port uint16) {
	if d.cs == nil {
		return
	}
	cs, err := d.cs.FindByID(ctx, commandStationID)
	if err != nil {
		return
	}
	return cs.WithrottleServerEnabled, cs.EffectiveWithrottleInboundPort(),
		cs.Z21ServerEnabled, cs.EffectiveZ21InboundPort()
}

// StartDccBus starts the existing dcc-bus microinit program without
// rewriting config (unlike EnsureRunning, which upserts the program).
func (d *DccBusService) StartDccBus(ctx context.Context, layoutID, commandStationID uint) error {
	return d.controlDccBus(ctx, layoutID, commandStationID, "start")
}

// StopDccBus stops the dcc-bus microinit program without removing it
// from config (unlike Stop, which also RemoveProgram).
func (d *DccBusService) StopDccBus(ctx context.Context, layoutID, commandStationID uint) error {
	return d.controlDccBus(ctx, layoutID, commandStationID, "stop")
}

// RestartDccBus stops then starts the dcc-bus microinit program.
func (d *DccBusService) RestartDccBus(ctx context.Context, layoutID, commandStationID uint) error {
	return d.controlDccBus(ctx, layoutID, commandStationID, "restart")
}

// controlDccBus is the shared implementation of Start/Stop/RestartDccBus.
func (d *DccBusService) controlDccBus(
	ctx context.Context,
	layoutID, commandStationID uint,
	action string,
) error {
	if d.mgr == nil {
		return ErrServiceManagerNotWired
	}
	name := programName(layoutID, commandStationID)
	fields := logrus.Fields{
		"program":          name,
		"layoutId":         layoutID,
		"commandStationId": commandStationID,
		"action":           action,
	}
	if d.log != nil {
		d.log.WithFields(fields).Debug("dcc-bus service action")
	}
	var err error
	switch action {
	case "start":
		err = d.mgr.StartService(ctx, name)
	case "stop":
		err = d.mgr.StopService(ctx, name)
	case "restart":
		err = d.mgr.RestartService(ctx, name)
	default:
		return fmt.Errorf("dcc-bus: unknown action %q", action)
	}
	if err != nil && d.log != nil {
		d.log.WithError(err).WithFields(fields).Warn("dcc-bus service action failed")
	}
	return err
}

// parseDccBusProgramLayoutID extracts layoutID from a bare program name
// "dcc-bus-{layoutID}-{commandStationID}". Returns false when the name
// does not match that shape for the given commandStationID.
func parseDccBusProgramLayoutID(bareName string, commandStationID uint) (uint, bool) {
	const prefix = "dcc-bus-"
	if !strings.HasPrefix(bareName, prefix) {
		return 0, false
	}
	rest := bareName[len(prefix):]
	suffix := fmt.Sprintf("-%d", commandStationID)
	if !strings.HasSuffix(rest, suffix) {
		return 0, false
	}
	layoutPart := rest[:len(rest)-len(suffix)]
	if layoutPart == "" {
		return 0, false
	}
	id, err := strconv.ParseUint(layoutPart, 10, 32)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// EnsureRunning guarantees a `dcc-bus-<L>-<C>` program exists in
// microinit, RUNNING, and accepting WS connections. It returns
// the loopback port the daemon listens on plus the program name
// for audit / logging.
//
// The lazy-spawn flow per §7e.6:
//
//  1. If a port is already allocated, just verify the daemon is
//     RUNNING and dial-able; return its port.
//  2. Otherwise allocate a free port from the pool, upsert the
//     program into microinit (autostart + autorestart), wait for
//     RUNNING and `tcp dial OK`, persist the port assignment.
func (d *DccBusService) EnsureRunning(ctx context.Context, layoutID, commandStationID uint) (uint16, string, error) {
	start := time.Now()
	port, name, err := d.ensureRunning(ctx, layoutID, commandStationID)
	if d.metrics != nil {
		d.metrics.RecordDccBusEnsureRunning(layoutID, commandStationID, time.Since(start), err)
	}
	return port, name, err
}

func (d *DccBusService) ensureRunning(ctx context.Context, layoutID, commandStationID uint) (uint16, string, error) {
	if d.mgr == nil {
		return 0, "", ErrServiceManagerNotWired
	}
	name := programName(layoutID, commandStationID)
	key := portKey{LayoutID: layoutID, CommandStationID: commandStationID}

	d.mu.Lock()
	port, ok := d.ports[key]
	d.mu.Unlock()

	if ok {
		if err := d.waitDialable(ctx, port); err == nil {
			d.log.WithFields(logrus.Fields{
				"program": name, "layoutId": layoutID, "commandStationId": commandStationID, "port": port,
			}).Debug("dcc-bus ensure running: already up")
			return port, name, nil
		}
		d.log.WithFields(logrus.Fields{
			"program": name, "port": port,
		}).Warn("dcc-bus ensure running: port assigned but not dialable, re-upserting")
	}

	if !ok {
		var err error
		port, err = d.allocatePortLocked(layoutID, commandStationID)
		if err != nil {
			return 0, name, err
		}
		d.log.WithFields(logrus.Fields{
			"program": name, "layoutId": layoutID, "commandStationId": commandStationID, "port": port,
		}).Info("dcc-bus ensure running: allocated port")
	}

	spec, err := d.buildServiceDef(ctx, name, layoutID, commandStationID, port)
	if err != nil {
		return 0, name, err
	}
	d.log.WithFields(logrus.Fields{"program": name, "port": port}).Info("dcc-bus ensure running: upserting microinit program")
	if err := d.mgr.UpsertService(ctx, GroupDccBus, spec); err != nil {
		return 0, name, fmt.Errorf("upsert dcc-bus program: %w", err)
	}
	if err := waitServiceKnown(ctx, d.mgr, name, 2*time.Second); err != nil {
		return 0, name, err
	}

	// microinit autostarts the program; explicitly StartProgram
	// covers the "already declared, was stopped" path.
	if err := d.mgr.StartService(ctx, name); err != nil {
		// microinit returns an error when the program is already
		// running; swallow that case.
		if !strings.Contains(err.Error(), "ALREADY_STARTED") &&
			!strings.Contains(strings.ToUpper(err.Error()), "ALREADY") {
			d.log.WithError(err).WithField("program", name).Debug("dcc-bus start (treat as soft)")
		}
	}

	if err := d.waitDialable(ctx, port); err != nil {
		d.log.WithError(err).WithFields(logrus.Fields{"program": name, "port": port}).Error("dcc-bus ensure running: daemon not dialable")
		return 0, name, fmt.Errorf("%w: %v", ErrDccBusUnavailable, err)
	}
	d.persistPort(ctx, key, port)
	d.log.WithFields(logrus.Fields{"program": name, "port": port}).Info("dcc-bus ensure running: daemon ready")
	return port, name, nil
}

// Stop tears down one daemon (e.g. when the operator detaches the
// command station from the layout). Idempotent.
func (d *DccBusService) Stop(ctx context.Context, layoutID, commandStationID uint) error {
	name := programName(layoutID, commandStationID)
	d.log.WithFields(logrus.Fields{
		"program": name, "layoutId": layoutID, "commandStationId": commandStationID,
	}).Info("dcc-bus stop: removing microinit program")
	if d.mgr == nil {
		return ErrServiceManagerNotWired
	}
	_ = d.mgr.StopService(ctx, name)
	if err := d.mgr.RemoveService(ctx, GroupDccBus, name); err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	key := portKey{LayoutID: layoutID, CommandStationID: commandStationID}
	d.mu.Lock()
	delete(d.ports, key)
	d.mu.Unlock()
	if d.redis != nil {
		_ = d.redis.Client().HDel(ctx, contract.DccBusPortsKey, contract.DccBusPortsField(layoutID, commandStationID)).Err()
	}
	return nil
}

// PublishCommand emits a typed envelope on the daemon's command
// channel. Returns an error when the daemon has never been spawned
// (no port assigned yet) so callers can decide whether to spawn
// lazily or surface "no_command_station".
func (d *DccBusService) PublishCommand(ctx context.Context, layoutID, commandStationID uint, eventType string, payload any) error {
	if d.redis == nil {
		return errors.New("dcc-bus: redis is not wired")
	}
	env, err := protocol.Frame(eventType, payload)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	channel := contract.DccBusCommandChannel(layoutID, commandStationID)
	return d.redis.Client().Publish(ctx, channel, raw).Err()
}

// ProxyEnabled reports whether the WS layer should advertise the
// reverse-proxy path (true) or the raw `127.0.0.1:<port>` address
// (false, dev only).
func (d *DccBusService) ProxyEnabled() bool { return d.cfg.ProxyEnabled }

// Executable returns the loco-server binary path used to spawn dcc-bus.
func (d *DccBusService) Executable() string { return d.cfg.Executable }

// allocatePortLocked picks the next free port from the pool. Caller
// MUST NOT hold the mutex.
func (d *DccBusService) allocatePortLocked(layoutID, commandStationID uint) (uint16, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	used := make(map[uint16]struct{}, len(d.ports))
	for _, p := range d.ports {
		used[p] = struct{}{}
	}
	for p := d.cfg.PortMin; p <= d.cfg.PortMax; p++ {
		if _, taken := used[p]; taken {
			continue
		}
		// Optimistically claim. The OS-level bind happens later via
		// microinit; on a race the daemon would fail to listen and
		// EnsureRunning would surface ErrDccBusUnavailable.
		d.ports[portKey{LayoutID: layoutID, CommandStationID: commandStationID}] = p
		return p, nil
	}
	return 0, ErrNoDccBusPortsAvailable
}

// persistPort mirrors the in-memory port assignment into Redis so a
// loco-server restart can hydrate the same map.
func (d *DccBusService) persistPort(ctx context.Context, k portKey, port uint16) {
	if d.redis == nil {
		return
	}
	field := contract.DccBusPortsField(k.LayoutID, k.CommandStationID)
	if err := d.redis.Client().HSet(ctx, contract.DccBusPortsKey, field, strconv.Itoa(int(port))).Err(); err != nil {
		d.log.WithError(err).Debug("dcc-bus persist port")
	}
}

// waitDialable polls the daemon's loopback port until a TCP dial
// succeeds or the SpawnTimeout budget elapses.
func (d *DccBusService) waitDialable(ctx context.Context, port uint16) error {
	deadline := time.Now().Add(d.cfg.SpawnTimeout)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	for {
		dialCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", addr)
		cancel()
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("dial dcc-bus on %s: %w", addr, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// buildServiceDef assembles the shell-safe microinit service definition.
func (d *DccBusService) buildServiceDef(ctx context.Context, name string, layoutID, commandStationID uint, port uint16) (microinit.ServiceDef, error) {
	if d.cs == nil {
		return microinit.ServiceDef{}, errors.New("dcc-bus: command station repository is not wired")
	}
	cs, err := d.cs.FindByID(ctx, commandStationID)
	if err != nil {
		return microinit.ServiceDef{}, fmt.Errorf("load command station %d: %w", commandStationID, err)
	}
	var layout domain.Layout
	if d.layouts != nil {
		layout, err = d.layouts.FindByID(ctx, layoutID)
		if err != nil {
			return microinit.ServiceDef{}, fmt.Errorf("load layout %d: %w", layoutID, err)
		}
	}
	args := []string{
		d.cfg.Executable, "dcc-bus",
		"--layout-id", strconv.FormatUint(uint64(layoutID), 10),
		"--command-station-id", strconv.FormatUint(uint64(commandStationID), 10),
		"--port", strconv.Itoa(int(port)),
		"--bind", "127.0.0.1",
		"--redis-addr", d.cfg.RedisAddr,
		"--jwt-secret", string(d.cfg.JWTSecret),
	}
	args = dccbuscli.AppendStationFlags(args, cs)
	args = dccbuscli.AppendLeaseFlags(args, layout, cs)
	if cs.Z21ServerEnabled {
		args = append(args, "--enable-z21")
		args = append(args, "--z21-port", strconv.FormatUint(uint64(cs.EffectiveZ21InboundPort()), 10))
	}
	if cs.Z21IPStickiness {
		args = append(args, "--z21-ip-stickiness")
	}
	if cs.WithrottleServerEnabled {
		args = append(args, "--enable-withrottle")
		args = append(args, "--withrottle-port", strconv.FormatUint(uint64(cs.EffectiveWithrottleInboundPort()), 10))
		args = append(args, "--withrottle-pairing-addr", strconv.FormatUint(uint64(cs.EffectiveWithrottlePairingAddr()), 10))
		args = append(args, "--withrottle-heartbeat-secs", strconv.FormatFloat(cs.EffectiveWithrottleHeartbeatSecs(), 'f', -1, 64))
	}
	if cs.BootStopEnabled {
		args = append(args, "--"+dccbuscli.FlagBootStopEnabled)
	}
	if cs.SingleVehicleControl {
		args = append(args, "--"+dccbuscli.FlagSingleVehicleControl)
	}
	if cs.Programming {
		args = append(args, "--"+dccbuscli.FlagEnableProgramming)
	}
	args = append(args, "--"+dccbuscli.FlagDefaultProgrammingTrack, cs.EffectiveDefaultProgrammingTrackOutput())
	for _, origin := range d.cfg.AllowedOrigins {
		args = append(args, "--allowed-origin", origin)
	}
	args = appendDccBusTelemetryArgs(args, d.cfg)
	for i, arg := range args {
		args[i] = microinit.ShellQuote(arg)
	}
	svc := microinit.ServiceDef{
		Name:             name,
		Enabled:          microinit.BoolPtr(true),
		Daemon:           microinit.BoolPtr(true),
		RestartPolicy:    microinit.RestartOnError,
		StartWaitSecs:    microinit.IntPtr(1),
		ShutdownWaitSecs: microinit.IntPtr(35),
		OrderPriority:    microinit.IntPtr(350),
		StartCmd:         "exec " + strings.Join(args, " "),
	}
	if d.cfg.ManagedRedis {
		svc.DependsOn = []string{"redis"}
	}
	return microinit.WithCreatedBy(svc, microinit.CreatedByBigfred), nil
}

func appendDccBusTelemetryArgs(args []string, cfg DccBusConfig) []string {
	if !cfg.EnableTelemetry || cfg.OTLPEndpoint == "" {
		return args
	}
	return append(args, "--enable-telemetry", "--otel-endpoint", cfg.OTLPEndpoint)
}
