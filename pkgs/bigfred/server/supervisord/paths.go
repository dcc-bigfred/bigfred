package supervisord

import "github.com/keskad/loco/pkgs/bigfred/server/datadir"

// Default hub runtime paths for loco-server supervisord (RW partition).
func DefaultConfigDir() string  { return datadir.Path("etc", "supervisord") }
func DefaultConfigFile() string { return datadir.Path("etc", "supervisord", "supervisord.conf") }
func DefaultInetHTTPAddr() string { return "127.0.0.1:9001" }
func DefaultPIDFile() string    { return datadir.Path("run", "supervisord.pid") }
func DefaultLogDir() string     { return datadir.Path("logs") }

// Paths holds filesystem locations and the inet HTTP control address for
// the managed supervisord instance.
type Paths struct {
	ConfigDir    string
	ConfigPath   string
	InetHTTPAddr string
	PIDFile      string
	LogDir       string
}

// DefaultPaths returns hub paths under the data directory (config, logs,
// pidfile) plus the loopback inet HTTP address for supervisorctl.
func DefaultPaths() (Paths, error) {
	return Paths{
		ConfigDir:    DefaultConfigDir(),
		ConfigPath:   DefaultConfigFile(),
		InetHTTPAddr: DefaultInetHTTPAddr(),
		PIDFile:      DefaultPIDFile(),
		LogDir:       DefaultLogDir(),
	}, nil
}
