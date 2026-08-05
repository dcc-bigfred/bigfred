package service

import (
	"errors"
	"fmt"
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

// SystemControl talks to microinit for host power control.
// A nil control (or nil underlying manager) returns ErrSystemUnavailable.
type SystemControl struct {
	mgr *manager
}

// NewSystemControl wraps a ServiceManager. Non-*manager implementations
// (tests / --no-supervisor) yield a control that always returns unavailable.
func NewSystemControl(mgr ServiceManager) *SystemControl {
	m, _ := mgr.(*manager)
	return &SystemControl{mgr: m}
}

// Info returns microinit mode and whether machine poweroff/reboot is allowed.
func (s *SystemControl) Info() (SystemInfo, error) {
	if s == nil || s.mgr == nil || s.mgr.supervisor == nil {
		return SystemInfo{}, ErrSystemUnavailable
	}
	info, err := s.mgr.supervisor.Client().Info()
	if err != nil {
		return SystemInfo{}, fmt.Errorf("%w: %v", ErrSystemUnavailable, err)
	}
	mode := info.Mode
	if mode == "" {
		mode = "supervise"
	}
	return SystemInfo{
		Mode:        mode,
		CanShutdown: mode == "init",
	}, nil
}

// RequestShutdown sends poweroff or reboot to microinit when mode is init.
func (s *SystemControl) RequestShutdown(mode string) error {
	switch mode {
	case "poweroff", "reboot":
	default:
		return fmt.Errorf("%w: %q", ErrInvalidShutdownMode, mode)
	}
	if s == nil || s.mgr == nil || s.mgr.supervisor == nil {
		return ErrSystemUnavailable
	}
	info, err := s.mgr.supervisor.Client().Info()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSystemUnavailable, err)
	}
	if info.Mode != "init" {
		return ErrSystemNotInit
	}
	return s.mgr.supervisor.Client().ShutdownMode(mode)
}
