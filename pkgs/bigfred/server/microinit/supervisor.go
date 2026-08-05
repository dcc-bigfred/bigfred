package microinit

import (
	"context"
	"errors"

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

const (
	RestartAlways  = config.RestartAlways
	RestartOnError = config.RestartOnError
	RestartNone    = config.RestartNone
)

// ShellQuote wraps an argv token for safe interpolation into a shell command
// line. Re-exported so service builders can use a single import.
func ShellQuote(value string) string { return config.ShellQuote(value) }

// BuildStartCmd joins and shell-quotes argv into a single "exec ..." command.
func BuildStartCmd(args []string) string { return config.BuildStartCmd(args) }

// LabelCreatedBy is the conventional drop-in ownership label key.
const LabelCreatedBy = config.LabelCreatedBy

// CreatedByBigfred is the created-by value for services written by bigfred.
const CreatedByBigfred = "bigfred"

func WithCreatedBy(svc ServiceDef, who string) ServiceDef {
	return config.WithCreatedBy(svc, who)
}

func MatchLabels(have, want map[string]string) bool {
	return config.MatchLabels(have, want)
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

// bigfredLabelSelector matches services created by this process.
func bigfredLabelSelector() map[string]string {
	return map[string]string{LabelCreatedBy: CreatedByBigfred}
}

// IsOurs reports whether labels mark a bigfred-managed service.
func IsOurs(labels map[string]string) bool {
	return MatchLabels(labels, bigfredLabelSelector())
}

// Supervisor embeds microinit via the shared SDK. Ownership is expressed with
// the created-by=bigfred label on ServiceDefs, not an in-memory map.
type Supervisor struct {
	Socket, Bin, ConfigPath, DropinDir string
	Log                                *logrus.Logger

	host *supervise.Host
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
	}
}

func (s *Supervisor) Client() *miclient.Client { return s.host.Client() }

// OwnsDropin reports whether a drop-in file exists for group/name.
func (s *Supervisor) OwnsDropin(group, name string) bool {
	return DropinExists(s.DropinDir, group, name)
}

func (s *Supervisor) EnsureRunning(ctx context.Context) (bool, error) {
	return s.host.EnsureRunning(ctx)
}

func (s *Supervisor) WriteDropin(group, name string, svc ServiceDef) error {
	return WriteDropin(s.DropinDir, group, name, WithCreatedBy(svc, CreatedByBigfred))
}

func (s *Supervisor) RemoveDropin(group, name string) error {
	return RemoveDropin(s.DropinDir, group, name)
}

func (s *Supervisor) SyncGroup(group string, desired map[string]ServiceDef) error {
	stamped := make(map[string]ServiceDef, len(desired))
	for name, svc := range desired {
		stamped[name] = WithCreatedBy(svc, CreatedByBigfred)
	}
	return SyncGroup(s.DropinDir, group, stamped)
}

// Status looks up one service on the control socket (nil if absent).
func (s *Supervisor) Status(name string) (*ServiceStatus, error) {
	st, err := s.Client().Status(name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return st, nil
}

// CanManage reports whether bigfred may write/overwrite a drop-in for name:
// the service is absent from microinit, or already labeled created-by=bigfred.
func (s *Supervisor) CanManage(name string) (bool, error) {
	st, err := s.Status(name)
	if err != nil {
		return false, err
	}
	if st == nil {
		return true, nil
	}
	return IsOurs(st.Labels), nil
}

// Stop stops every service labeled created-by=bigfred, then shuts down the
// microinit process only if this host spawned it.
func (s *Supervisor) Stop(ctx context.Context) error {
	list, err := s.Client().List()
	if err == nil {
		want := bigfredLabelSelector()
		for _, svc := range list {
			if MatchLabels(svc.Labels, want) {
				_ = s.Client().Control(svc.Name, "stop")
			}
		}
	}
	return s.host.Shutdown(ctx)
}
