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
	GroupBigfred = "bigfred"
	GroupInfra   = "infra"
)

var (
	ErrNotFound = microinit.ErrNotFound
	// ErrSystemService is returned when refusing to write a drop-in for a
	// service declared in the main microinit.json.
	ErrSystemService = errors.New("refusing to manage system microinit service")
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
	// IsSystemService reports whether name is declared in main microinit.json.
	IsSystemService(context.Context, string) (bool, error)
}

type MicroinitConfig struct {
	Socket, Bin, ConfigPath, DropinDir string
	Log                                *logrus.Logger
}

type manager struct{ supervisor *microinit.Supervisor }

func NewMicroinitManager(cfg MicroinitConfig) (ServiceManager, error) {
	if cfg.ConfigPath == "" || cfg.DropinDir == "" {
		return nil, errors.New("microinit config path and drop-in directory are required")
	}
	return &manager{supervisor: microinit.NewSupervisor(cfg.Socket, cfg.Bin, cfg.ConfigPath, cfg.DropinDir, cfg.Log)}, nil
}

func (m *manager) Start(ctx context.Context) error {
	_, err := m.supervisor.EnsureRunning(ctx)
	return err
}

func (m *manager) Stop(ctx context.Context) error { return m.supervisor.Stop(ctx) }

func (m *manager) IsSystemService(_ context.Context, name string) (bool, error) {
	names, err := m.supervisor.SystemServiceNames()
	if err != nil {
		return false, err
	}
	_, ok := names[name]
	return ok, nil
}

func (m *manager) UpsertService(ctx context.Context, group string, svc microinit.ServiceDef) error {
	sys, err := m.IsSystemService(ctx, svc.Name)
	if err != nil {
		return err
	}
	if sys {
		return fmt.Errorf("%w: %s", ErrSystemService, svc.Name)
	}
	return m.supervisor.WriteDropin(group, svc.Name, svc)
}

func (m *manager) ReplaceServices(_ context.Context, group string, services []microinit.ServiceDef) error {
	system, err := m.supervisor.SystemServiceNames()
	if err != nil {
		return err
	}
	desired := make(map[string]microinit.ServiceDef, len(services))
	for _, svc := range services {
		if _, sys := system[svc.Name]; sys {
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
	rows, err := m.supervisor.Client().List()
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Name == name {
			return true, nil
		}
	}
	return false, nil
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

// EnsureInfra writes redis/alloy drop-ins only when those services are not
// system-declared and not already provided by another microinit source.
// Leftover bigfred drop-ins for system services are removed so base config wins.
func EnsureInfra(ctx context.Context, mgr ServiceManager, cfg InfraConfig) error {
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
			return err
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
	sys, err := mgr.IsSystemService(ctx, name)
	if err != nil {
		return err
	}
	if sys {
		// Drop any previous bigfred override so system definition is authoritative.
		_ = mgr.RemoveService(ctx, group, name)
		return nil
	}

	exists, err := mgr.HasService(ctx, name)
	if err != nil {
		return err
	}
	_, dropinDir := mgr.Paths()
	ours := microinit.DropinExists(dropinDir, group, name)
	if exists && !ours {
		// Provided by another drop-in / source — do not steal.
		return nil
	}

	svc, err := build()
	if err != nil {
		return err
	}
	if err := mgr.UpsertService(ctx, group, svc); err != nil {
		return err
	}
	return mgr.StartService(ctx, name)
}
