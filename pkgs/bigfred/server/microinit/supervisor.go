package microinit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Supervisor struct {
	Socket, Bin, ConfigPath, DropinDir string
	Log                                *logrus.Logger

	spawned bool
	cmd     *exec.Cmd
	waitCh  <-chan error
	mu      sync.Mutex
	owned   map[string]struct{}
	client  *Client
}

func NewSupervisor(socket, bin, configPath, dropinDir string, log *logrus.Logger) *Supervisor {
	if socket == "" {
		socket = DefaultSocketPath()
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
		return false, fmt.Errorf("microinit process is running but IPC is unavailable (socket %s)", s.Socket)
	}
	if err := os.MkdirAll(s.DropinDir, 0o755); err != nil {
		return false, fmt.Errorf("create microinit drop-in dir %s: %w", s.DropinDir, err)
	}
	if err := os.MkdirAll(filepath.Dir(s.ConfigPath), 0o755); err != nil {
		return false, fmt.Errorf("create microinit config dir %s: %w", filepath.Dir(s.ConfigPath), err)
	}
	if sockDir := filepath.Dir(s.Socket); sockDir != "" && sockDir != "." {
		if err := os.MkdirAll(sockDir, 0o755); err != nil {
			return false, fmt.Errorf("create microinit socket dir %s: %w", sockDir, err)
		}
	}
	if _, err := os.Stat(s.ConfigPath); os.IsNotExist(err) {
		content, marshalErr := json.Marshal(map[string]any{"services": []any{}, "socket": s.Socket})
		if marshalErr != nil {
			return false, marshalErr
		}
		if err := writeFileAtomically(s.ConfigPath, append(content, '\n')); err != nil {
			return false, fmt.Errorf("write microinit config %s: %w", s.ConfigPath, err)
		}
	} else if err != nil {
		return false, fmt.Errorf("stat microinit config %s: %w", s.ConfigPath, err)
	}
	var logBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, s.Bin, "--socket", s.Socket, "supervise", "--config", s.ConfigPath)
	cmd.Stdout = &logBuf
	cmd.Stderr = &logBuf
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("start microinit (%s --socket %s supervise --config %s): %w", s.Bin, s.Socket, s.ConfigPath, err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	s.cmd, s.spawned, s.waitCh = cmd, true, waitCh
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
			<-waitCh
			s.spawned, s.cmd, s.waitCh = false, nil, nil
			return false, ctx.Err()
		case waitErr := <-waitCh:
			s.spawned, s.cmd, s.waitCh = false, nil, nil
			detail := strings.TrimSpace(logBuf.String())
			if detail == "" {
				detail = fmt.Sprintf("exit: %v", waitErr)
			}
			return false, fmt.Errorf("microinit exited before ready (bin=%s socket=%s config=%s): %s", s.Bin, s.Socket, s.ConfigPath, detail)
		case <-deadline.C:
			_ = cmd.Process.Kill()
			<-waitCh
			s.spawned, s.cmd, s.waitCh = false, nil, nil
			detail := strings.TrimSpace(logBuf.String())
			if detail == "" {
				detail = "no output captured"
			}
			return false, fmt.Errorf("microinit did not become ready within 10s (bin=%s socket=%s config=%s): %s", s.Bin, s.Socket, s.ConfigPath, detail)
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
	// Only adjust ownership for services in this group — never wipe
	// infra (or other groups) when syncing bigfred dcc-bus services.
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
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	owned := make([]string, 0, len(s.owned))
	for name := range s.owned {
		owned = append(owned, name)
	}
	spawned, cmd, waitCh := s.spawned, s.cmd, s.waitCh
	s.mu.Unlock()
	for _, name := range owned {
		_ = s.client.Control(name, "stop")
	}
	if !spawned || cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = s.client.Shutdown()
	if waitCh == nil {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		waitCh = done
	}
	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-waitCh
		return ctx.Err()
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		<-waitCh
		return fmt.Errorf("microinit shutdown timed out (socket %s)", s.Socket)
	case <-waitCh:
	}
	s.mu.Lock()
	s.spawned, s.cmd, s.waitCh = false, nil, nil
	s.mu.Unlock()
	return nil
}
