package service

import (
	"errors"
	"fmt"
	"net"
	"time"

	miclient "github.com/dcc-bigfred/microinit/go/client"
)

var (
	// ErrSystemUnavailable is returned when microinit is not wired
	// (--no-supervisor) or the control socket cannot be reached.
	ErrSystemUnavailable = errors.New("system control unavailable")
	// ErrSystemNotInit is returned when machine poweroff/reboot is
	// requested but microinit is running in supervise mode.
	ErrSystemNotInit = errors.New("system shutdown requires microinit init mode")
	// ErrInvalidShutdownMode is returned for unknown shutdown modes.
	ErrInvalidShutdownMode = errors.New("invalid shutdown mode")
)

// SystemInfo is the admin-facing snapshot of host power capability.
type SystemInfo struct {
	Mode        string `json:"mode"`
	CanShutdown bool   `json:"canShutdown"`
}

// SystemPorts reports whether optional local admin UIs are reachable.
type SystemPorts struct {
	OsUI    bool `json:"osUI"`
	Grafana bool `json:"grafana"`
}

// MicroinitPower is the subset of the microinit Go client used for host power.
type MicroinitPower interface {
	Info() (*miclient.DaemonInfo, error)
	ShutdownMode(mode string) error
}

// SystemControl talks to microinit for host power control.
type SystemControl struct {
	power MicroinitPower
}

// NewSystemControl wraps a ServiceManager when it is the microinit-backed
// *manager with a live supervisor; otherwise returns a control that always
// reports unavailable (tests / --no-supervisor).
func NewSystemControl(mgr ServiceManager) *SystemControl {
	m, ok := mgr.(*manager)
	if !ok || m == nil || m.supervisor == nil {
		return &SystemControl{}
	}
	return &SystemControl{power: m.supervisor.Client()}
}

// NewSystemControlWithPower is for tests that inject a fake microinit client.
func NewSystemControlWithPower(power MicroinitPower) *SystemControl {
	return &SystemControl{power: power}
}

// Info returns microinit mode and whether machine poweroff/reboot is allowed.
func (s *SystemControl) Info() (SystemInfo, error) {
	if s == nil || s.power == nil {
		return SystemInfo{}, ErrSystemUnavailable
	}
	info, err := s.power.Info()
	if err != nil {
		return SystemInfo{}, fmt.Errorf("%w: %v", ErrSystemUnavailable, err)
	}
	mode := normalizeDaemonMode(info.Mode)
	return SystemInfo{
		Mode:        mode,
		CanShutdown: mode == "init",
	}, nil
}

// RequestShutdown sends poweroff or reboot to microinit when mode is init.
// Halt is intentionally rejected here — BigFred admin UI only offers
// poweroff/reboot; direct SDK callers may use Client.ShutdownMode("halt").
func (s *SystemControl) RequestShutdown(mode string) error {
	switch mode {
	case "poweroff", "reboot":
	default:
		return fmt.Errorf("%w: %q", ErrInvalidShutdownMode, mode)
	}
	if s == nil || s.power == nil {
		return ErrSystemUnavailable
	}
	info, err := s.power.Info()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSystemUnavailable, err)
	}
	// Empty Mode (older microinit without the field) is treated as supervise:
	// refuse host power rather than assume init. Same rule as Info().
	if normalizeDaemonMode(info.Mode) != "init" {
		return ErrSystemNotInit
	}
	return s.power.ShutdownMode(mode)
}

// Ports probes loopback TCP ports for bigfred-os-ui (8090) and Grafana (3000).
func (s *SystemControl) Ports() SystemPorts {
	return SystemPorts{
		OsUI:    tcpPortOpen("127.0.0.1:8090", 200*time.Millisecond),
		Grafana: tcpPortOpen("127.0.0.1:3000", 200*time.Millisecond),
	}
}

func tcpPortOpen(addr string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// normalizeDaemonMode maps wire values to "init" or "supervise".
// Unknown / empty → supervise (safe side: deny host power).
func normalizeDaemonMode(mode string) string {
	if mode == "init" {
		return "init"
	}
	return "supervise"
}
