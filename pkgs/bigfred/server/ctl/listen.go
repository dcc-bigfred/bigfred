package ctl

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

// ErrAlreadyRunning is returned by Listen when another process is
// already accepting on the socket path.
var ErrAlreadyRunning = errors.New("bigfred already running")

// DefaultSocket is $DATA_DIR/run/bigfred.sock.
func DefaultSocket() string {
	return datadir.Path("run", "bigfred.sock")
}

// Listen binds a Unix socket at path. A live peer is refused (the inode
// is not unlinked). A leftover .sock after a crash is replaced.
func Listen(path string) (net.Listener, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("ctl socket dir: %w", err)
		}
	}

	conn, err := net.Dial("unix", path)
	if err == nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w at %s", ErrAlreadyRunning, path)
	}
	if !isStaleDial(err) {
		return nil, fmt.Errorf("ctl socket probe %s: %w", path, err)
	}
	if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
		return nil, fmt.Errorf("ctl unlink stale %s: %w", path, rmErr)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("ctl listen %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("ctl chmod %s: %w", path, err)
	}
	return ln, nil
}

func isStaleDial(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return isStaleDial(op.Err)
	}
	var sys *os.SyscallError
	if errors.As(err, &sys) {
		return isStaleDial(sys.Err)
	}
	return false
}
