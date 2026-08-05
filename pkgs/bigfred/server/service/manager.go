package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/microinit"
)

const (
	GroupDccBus = "dcc-bus"
	GroupInfra  = "infra"
)

var (
	ErrNotFound = microinit.ErrNotFound
	// ErrNotOurs is returned when refusing to write a drop-in for a service
	// that already exists without created-by=bigfred.
	ErrNotOurs = errors.New("refusing to manage microinit service not labeled created-by=bigfred")
)

type ServiceState struct {
	Name     string
	State    string
	PID      int
	Restarts uint32
	Enabled  bool
}

type ServiceManager interface {
	Start(context.Context) error
	Stop(context.Context) error
	UpsertService(context.Context, string, microinit.ServiceDef) error
	ReplaceServices(context.Context, string, []microinit.ServiceDef) error
	RemoveService(context.Context, string, string) error
	StartService(context.Context, string) error
	StopService(context.Context, string) error
	RestartService(context.Context, string) error
	Status(context.Context) ([]ServiceState, error)
	RunHealthLoop(context.Context, time.Duration, func([]ServiceState))
	Paths() (socket, dropinDir string)
	HasService(context.Context, string) (bool, error)
	// CanManage reports whether bigfred may write/overwrite a drop-in for name
	// (service absent, or already labeled created-by=bigfred).
	CanManage(context.Context, string) (bool, error)
}

type MicroinitConfig struct {
	Socket, Bin, ConfigPath, DropinDir string
	// SpawnEnv is passed only to a supervised microinit child (not the embedder).
	SpawnEnv []string
	Log      *logrus.Logger
}

type manager struct {
	supervisor *microinit.Supervisor
	log        *logrus.Logger
}

func NewMicroinitManager(cfg MicroinitConfig) (ServiceManager, error) {
	if cfg.ConfigPath == "" || cfg.DropinDir == "" {
		return nil, errors.New("microinit config path and drop-in directory are required")
	}
	return &manager{
		supervisor: microinit.NewSupervisor(cfg.Socket, cfg.Bin, cfg.ConfigPath, cfg.DropinDir, cfg.SpawnEnv, cfg.Log),
		log:        cfg.Log,
	}, nil
}

func (m *manager) Start(ctx context.Context) error {
	_, err := m.supervisor.EnsureRunning(ctx)
	return err
}

func (m *manager) Stop(ctx context.Context) error { return m.supervisor.Stop(ctx) }

func (m *manager) CanManage(_ context.Context, name string) (bool, error) {
	return m.supervisor.CanManage(name)
}

func (m *manager) UpsertService(ctx context.Context, group string, svc microinit.ServiceDef) error {
	ok, err := m.CanManage(ctx, svc.Name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotOurs, svc.Name)
	}
	return m.supervisor.WriteDropin(group, svc.Name, svc)
}

func (m *manager) ReplaceServices(ctx context.Context, group string, services []microinit.ServiceDef) error {
	desired := make(map[string]microinit.ServiceDef, len(services))
	for _, svc := range services {
		ok, err := m.CanManage(ctx, svc.Name)
		if err != nil {
			return err
		}
		if !ok {
			if m.log != nil {
				m.log.WithField("service", svc.Name).
					Warn("microinit: skipping service not labeled created-by=bigfred")
			}
			continue
		}
		desired[svc.Name] = svc
	}
	return m.supervisor.SyncGroup(group, desired)
}

func (m *manager) RemoveService(_ context.Context, group, name string) error {
	return m.supervisor.RemoveDropin(group, name)
}

func (m *manager) StartService(_ context.Context, name string) error {
	return m.supervisor.Client().Control(name, "start")
}

func (m *manager) StopService(_ context.Context, name string) error {
	return m.supervisor.Client().Control(name, "stop")
}

func (m *manager) RestartService(_ context.Context, name string) error {
	return m.supervisor.Client().Control(name, "restart")
}

func (m *manager) Status(_ context.Context) ([]ServiceState, error) {
	rows, err := m.supervisor.Client().List()
	if err != nil {
		return nil, err
	}
	out := make([]ServiceState, 0, len(rows))
	for _, row := range rows {
		pid := 0
		if row.PID != nil {
			pid = int(*row.PID)
		}
		out = append(out, ServiceState{Name: row.Name, State: row.State, PID: pid, Restarts: row.Restarts, Enabled: row.Enabled})
	}
	return out, nil
}

func (m *manager) HasService(_ context.Context, name string) (bool, error) {
	st, err := m.supervisor.Status(name)
	if err != nil {
		return false, err
	}
	return st != nil, nil
}

func (m *manager) RunHealthLoop(ctx context.Context, interval time.Duration, onChange func([]ServiceState)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var previous map[string]string
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				states, err := m.Status(ctx)
				if err != nil {
					continue
				}
				next := map[string]string{}
				for _, state := range states {
					next[state.Name] = state.State
				}
				changed := len(next) != len(previous)
				if !changed {
					for name, state := range next {
						if previous[name] != state {
							changed = true
							break
						}
					}
				}
				if changed && onChange != nil {
					onChange(states)
				}
				previous = next
			}
		}
	}()
}

func (m *manager) Paths() (string, string) { return m.supervisor.Socket, m.supervisor.DropinDir }

type TelemetryConfig = microinit.TelemetryConfig
type RedisConfig = microinit.RedisConfig
type InfraConfig = microinit.InfraConfig
type RDBSavePoint = microinit.RDBSavePoint

const DefaultOTLPEndpoint = microinit.DefaultOTLPEndpoint

func DefaultTelemetryConfigPath() string            { return microinit.DefaultTelemetryConfigPath() }
func DefaultAlloyStoragePath() string               { return microinit.DefaultAlloyStoragePath() }
func AlloyRunConfigPath(cfg TelemetryConfig) string { return microinit.AlloyRunConfigPath(cfg) }
func BigFredAlloyGeneratedPath(cfg TelemetryConfig) string {
	return microinit.BigFredAlloyGeneratedPath(cfg)
}
func ResolveRDBSavePoints(noPersist bool, values []string) ([]RDBSavePoint, error) {
	return microinit.ResolveRDBSavePoints(noPersist, values)
}

// EnsureInfra writes redis/alloy drop-ins only when those services are absent
// or already labeled created-by=bigfred. Foreign services are left alone; any
// leftover bigfred drop-in for a foreign name is removed.
//
// Alloy setup is best-effort: failures are logged as warnings and do not fail
// bootstrap. Redis remains required when managed.
func EnsureInfra(ctx context.Context, mgr ServiceManager, log *logrus.Logger, cfg InfraConfig) error {
	if !cfg.Redis.Disable {
		if err := ensureOwnedInfra(ctx, mgr, GroupInfra, "redis", func() (microinit.ServiceDef, error) {
			return microinit.RedisServiceDef(cfg.Redis)
		}); err != nil {
			return err
		}
	}
	if cfg.Telemetry.Enable {
		if err := ensureOwnedInfra(ctx, mgr, GroupInfra, "alloy", func() (microinit.ServiceDef, error) {
			if err := microinit.PrepareAlloyTelemetry(cfg.Telemetry); err != nil {
				return microinit.ServiceDef{}, err
			}
			return microinit.AlloyServiceDef(cfg.Telemetry)
		}); err != nil {
			if log != nil {
				log.WithError(err).WithField("service", "alloy").Warn("microinit alloy setup failed; continuing without managed telemetry")
			}
		}
	}
	return nil
}

func ensureOwnedInfra(
	ctx context.Context,
	mgr ServiceManager,
	group, name string,
	build func() (microinit.ServiceDef, error),
) error {
	ok, err := mgr.CanManage(ctx, name)
	if err != nil {
		return err
	}
	if !ok {
		// Foreign (e.g. system image) — drop leftover bigfred override if any.
		_ = mgr.RemoveService(ctx, group, name)
		return nil
	}

	svc, err := build()
	if err != nil {
		return err
	}
	if err := mgr.UpsertService(ctx, group, svc); err != nil {
		return err
	}
	if err := waitServiceKnown(ctx, mgr, name, 2*time.Second); err != nil {
		return err
	}
	return mgr.StartService(ctx, name)
}

// waitServiceKnown polls microinit Status until name is registered (drop-in
// reload debounce is ~300ms) or ctx/timeout expires.
func waitServiceKnown(ctx context.Context, mgr ServiceManager, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		ok, err := mgr.HasService(ctx, name)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("microinit service %q not visible within %s after drop-in write", name, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
