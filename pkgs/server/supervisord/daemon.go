package supervisord

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Daemon manages the supervisord process itself.
type Daemon struct {
	Bin        string
	ConfigPath string
	PIDFile    string
}

// IsRunning reports whether pidfile points to a live process.
func (d *Daemon) IsRunning() (bool, int, error) {
	pid, err := readPIDFile(d.PIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	if pid <= 0 {
		return false, 0, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, nil
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid, nil
	}
	return true, pid, nil
}

// Start launches supervisord in the background.
func (d *Daemon) Start(ctx context.Context) error {
	running, _, err := d.IsRunning()
	if err != nil {
		return err
	}
	if running {
		return nil
	}

	bin := d.Bin
	if bin == "" {
		bin = "supervisord"
	}
	cmd := exec.CommandContext(ctx, bin, "-c", d.ConfigPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start supervisord: %w", err)
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

// WaitUntilReady polls supervisorctl until the daemon responds.
func (d *Daemon) WaitUntilReady(ctx context.Context, ctl *Ctl, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := ctl.Ping(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("supervisord not ready after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// WaitUntilStopped waits for the pidfile to disappear.
func (d *Daemon) WaitUntilStopped(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(d.PIDFile); os.IsNotExist(err) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("supervisord pidfile still present after %s", timeout)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// ForceStop sends SIGKILL to the pid from pidfile and removes the file.
func (d *Daemon) ForceStop() error {
	pid, err := readPIDFile(d.PIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if pid > 0 {
		proc, err := os.FindProcess(pid)
		if err == nil {
			_ = proc.Kill()
		}
	}
	return os.Remove(d.PIDFile)
}

func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty pidfile %s", path)
	}
	return strconv.Atoi(fields[0])
}

// EnsureDir creates dir with 0700 if missing.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o700)
}

// CurrentUserName returns the username for the [supervisord] user= directive.
func CurrentUserName() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// ConfigDirFromPath returns the directory containing configPath.
func ConfigDirFromPath(configPath string) string {
	return filepath.Dir(configPath)
}
