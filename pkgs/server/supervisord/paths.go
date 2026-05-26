package supervisord

import (
	"os"
	"path/filepath"
)

// Paths holds XDG-derived locations for a non-root supervisord instance.
type Paths struct {
	ConfigDir  string
	ConfigPath string
	SocketPath string
	PIDFile    string
	LogDir     string
}

// DefaultPaths returns paths under $XDG_RUNTIME_DIR/loco/supervisord/ and
// $XDG_CACHE_HOME/loco/supervisord/.
func DefaultPaths() (Paths, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return Paths{}, err
	}
	logDir := filepath.Join(cache, "loco", "supervisord")

	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = filepath.Join(logDir, "run")
	}
	configDir := filepath.Join(runtime, "loco", "supervisord")

	return Paths{
		ConfigDir:  configDir,
		ConfigPath: filepath.Join(configDir, "supervisord.conf"),
		SocketPath: filepath.Join(configDir, "supervisor.sock"),
		PIDFile:    filepath.Join(configDir, "supervisord.pid"),
		LogDir:     logDir,
	}, nil
}
