package microinit

import (
	miclient "github.com/dcc-bigfred/microinit/go/client"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

// Re-export shared client errors and helpers used by call sites that import
// this package (service manager, etc.). Prefer importing
// github.com/dcc-bigfred/microinit/go/client directly in new code.
var (
	ErrInvalidName   = miclient.ErrInvalidName
	ErrInvalidAction = miclient.ErrInvalidAction
	ErrNotFound      = miclient.ErrNotFound
)

// DefaultSocket is the hub path when the data root is /data.
const DefaultSocket = miclient.DefaultSocket

// DefaultSocketPath returns `$BIGFRED_DATA_DIR/run/microinit.sock` (or DATA_DIR /data).
func DefaultSocketPath() string {
	return datadir.Path("run", "microinit.sock")
}

// Client is the shared microinit IPC client.
type Client = miclient.Client

// ServiceStatus is a microinit list/status entry.
type ServiceStatus = miclient.ServiceStatus

func validateName(name string) error {
	return miclient.ValidateName(name)
}
