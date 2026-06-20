package supervisord

// Hub runtime paths for loco-server supervisord (RW partition on the hub image).
const (
	DefaultConfigDir  = "/data/etc/supervisord"
	DefaultConfigFile = "/data/etc/supervisord/supervisord.conf"
	DefaultSocketPath = "/data/run/supervisord.sock"
	DefaultPIDFile    = "/data/run/supervisord.pid"
	DefaultLogDir     = "/data/logs"
)

// Paths holds filesystem locations for the managed supervisord instance.
type Paths struct {
	ConfigDir  string
	ConfigPath string
	SocketPath string
	PIDFile    string
	LogDir     string
}

// DefaultPaths returns hub paths under /data (config, logs, unix socket, pidfile).
func DefaultPaths() (Paths, error) {
	return Paths{
		ConfigDir:  DefaultConfigDir,
		ConfigPath: DefaultConfigFile,
		SocketPath: DefaultSocketPath,
		PIDFile:    DefaultPIDFile,
		LogDir:     DefaultLogDir,
	}, nil
}
