package microinit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Supervisor struct {
	Socket, Bin, ConfigPath, DropinDir string
	Log                                *logrus.Logger

	spawned bool
	cmd     *exec.Cmd
	mu      sync.Mutex
	owned   map[string]struct{}
	client  *Client
}

func NewSupervisor(socket, bin, configPath, dropinDir string, log *logrus.Logger) *Supervisor {
	if socket == "" {
		socket = DefaultSocket
	}
	if bin == "" {
		bin = "microinit"
	}
	return &Supervisor{Socket: socket, Bin: bin, ConfigPath: configPath, DropinDir: dropinDir, Log: log, owned: map[string]struct{}{}, client: &Client{Socket: socket}}
}
func (s *Supervisor) Client() *Client { return s.client }

// EnsureRunning joins an existing microinit when its socket responds. Otherwise
// it launches exactly one host-supervisor instance and waits for its IPC socket.
func (s *Supervisor) EnsureRunning(ctx context.Context) (bool, error) {
	if _, err := s.client.List(); err == nil {
		return true, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.client.List(); err == nil {
		return true, nil
	}
	if s.spawned && s.cmd != nil && s.cmd.Process != nil {
		return false, fmt.Errorf("microinit process is running but IPC is unavailable")
	}
	if err := os.MkdirAll(s.DropinDir, 0o755); err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(s.ConfigPath), 0o755); err != nil {
		return false, err
	}
	if _, err := os.Stat(s.ConfigPath); os.IsNotExist(err) {
		content, marshalErr := json.Marshal(map[string]any{"services": []any{}, "socket": s.Socket})
		if marshalErr != nil {
			return false, marshalErr
		}
		if err := writeFileAtomically(s.ConfigPath, append(content, '\n')); err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}
	cmd := exec.CommandContext(ctx, s.Bin, "--socket", s.Socket, "supervise", "--config", s.ConfigPath)
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("start microinit: %w", err)
	}
	s.cmd, s.spawned = cmd, true
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := s.client.List(); err == nil {
			return false, nil
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return false, ctx.Err()
		case <-deadline.C:
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return false, fmt.Errorf("microinit did not become ready")
		case <-ticker.C:
		}
	}
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
	if err := SyncGroup(s.DropinDir, group, desired); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for name := range s.owned {
		if _, ok := desired[name]; !ok {
			delete(s.owned, name)
		}
	}
	for name := range desired {
		s.owned[name] = struct{}{}
	}
	return nil
}
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	owned := make([]string, 0, len(s.owned))
	for name := range s.owned {
		owned = append(owned, name)
	}
	spawned, cmd := s.spawned, s.cmd
	s.mu.Unlock()
	for _, name := range owned {
		_ = s.client.Control(name, "stop")
	}
	if !spawned || cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = s.client.Shutdown()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return ctx.Err()
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		return fmt.Errorf("microinit shutdown timed out")
	case <-done:
	}
	return nil
}
