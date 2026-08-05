package microinit

import (
	"context"
	"sync"

	miclient "github.com/dcc-bigfred/microinit/go/client"
	"github.com/dcc-bigfred/microinit/go/config"
	"github.com/dcc-bigfred/microinit/go/supervise"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

// Re-export shared client / config symbols used by call sites that import this
// package. Prefer importing …/go/{client,config,supervise} directly in new code.
var (
	ErrInvalidName   = miclient.ErrInvalidName
	ErrInvalidAction = miclient.ErrInvalidAction
	ErrNotFound      = miclient.ErrNotFound
)

const DefaultSocket = miclient.DefaultSocket

func DefaultSocketPath() string {
	return datadir.Path("run", "microinit.sock")
}

type (
	Client        = miclient.Client
	ServiceStatus = miclient.ServiceStatus
	ServiceDef    = config.ServiceDef
	LivenessProbe = config.LivenessProbe
	DropinFile    = config.DropinFile
)

func BoolPtr(v bool) *bool { return config.BoolPtr(v) }
func IntPtr(v int) *int    { return config.IntPtr(v) }

// LabelCreatedBy is the conventional drop-in ownership label key.
const LabelCreatedBy = config.LabelCreatedBy

// CreatedByBigfred is the created-by value for services written by bigfred.
const CreatedByBigfred = "bigfred"

func WithCreatedBy(svc ServiceDef, who string) ServiceDef {
	return config.WithCreatedBy(svc, who)
}

func WriteDropin(dir, group, name string, svc ServiceDef) error {
	return config.WriteDropin(dir, group, name, svc)
}
func RemoveDropin(dir, group, name string) error {
	return config.RemoveDropin(dir, group, name)
}
func SyncGroup(dir, group string, desired map[string]ServiceDef) error {
	return config.SyncGroup(dir, group, desired)
}
func ListGroup(dir, group string) (map[string]ServiceDef, error) {
	return config.ListGroup(dir, group)
}
func DropinExists(dir, group, name string) bool {
	return config.DropinExists(dir, group, name)
}
func BaseConfigServiceNames(configPath string) (map[string]struct{}, error) {
	return config.BaseConfigServiceNames(configPath)
}

// Supervisor embeds microinit via the shared SDK and adds bigfred policies:
// tracking drop-ins this process wrote, and stopping those services before
// tearing down a spawned daemon.
type Supervisor struct {
	Socket, Bin, ConfigPath, DropinDir string
	Log                                *logrus.Logger

	host  *supervise.Host
	owned map[string]struct{}
	mu    sync.Mutex
}

func NewSupervisor(socket, bin, configPath, dropinDir string, log *logrus.Logger) *Supervisor {
	if socket == "" {
		socket = DefaultSocketPath()
	}
	if bin == "" {
		bin = "microinit"
	}
	return &Supervisor{
		Socket:     socket,
		Bin:        bin,
		ConfigPath: configPath,
		DropinDir:  dropinDir,
		Log:        log,
		host:       supervise.New(socket, bin, configPath, dropinDir),
		owned:      map[string]struct{}{},
	}
}

func (s *Supervisor) Client() *miclient.Client { return s.host.Client() }

func (s *Supervisor) SystemServiceNames() (map[string]struct{}, error) {
	return BaseConfigServiceNames(s.ConfigPath)
}

// OwnsDropin reports whether a drop-in file exists for group/name.
func (s *Supervisor) OwnsDropin(group, name string) bool {
	return DropinExists(s.DropinDir, group, name)
}

func (s *Supervisor) EnsureRunning(ctx context.Context) (bool, error) {
	return s.host.EnsureRunning(ctx)
}

func (s *Supervisor) MarkOwned(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owned[name] = struct{}{}
}

func (s *Supervisor) IsOwned(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.owned[name]
	return ok
}

func (s *Supervisor) WriteDropin(group, name string, svc ServiceDef) error {
	if err := WriteDropin(s.DropinDir, group, name, svc); err != nil {
		return err
	}
	s.MarkOwned(name)
	return nil
}

func (s *Supervisor) RemoveDropin(group, name string) error {
	if err := RemoveDropin(s.DropinDir, group, name); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.owned, name)
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) SyncGroup(group string, desired map[string]ServiceDef) error {
	previous, err := ListGroup(s.DropinDir, group)
	if err != nil {
		return err
	}
	if err := SyncGroup(s.DropinDir, group, desired); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for name := range previous {
		if _, ok := desired[name]; !ok {
			delete(s.owned, name)
		}
	}
	for name := range desired {
		s.owned[name] = struct{}{}
	}
	return nil
}

// Stop applies bigfred exit policy: stop services this process marked owned,
// then shut down the microinit process only if we spawned it.
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	owned := make([]string, 0, len(s.owned))
	for name := range s.owned {
		owned = append(owned, name)
	}
	s.mu.Unlock()
	for _, name := range owned {
		_ = s.Client().Control(name, "stop")
	}
	return s.host.Shutdown(ctx)
}
