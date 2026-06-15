package supervisord

import "fmt"

// RedisConfig collects the few knobs InfraProcesses exposes for the
// managed redis-server child. Redis is mandatory in BigFred (state
// cache + cross-process pub/sub between loco-server and dcc-bus); the
// only way to skip the managed instance is to point loco-server at an
// external one with --redis-external, which sets RedisConfig.Disable.
type RedisConfig struct {
	// Bin is the absolute or PATH-relative redis-server binary.
	// Defaults to "redis-server" when empty.
	Bin string
	// BindAddr is the interface redis-server binds on. Defaults to
	// "127.0.0.1" — Redis carries privileged state (sessions, port
	// allocations, pub/sub), so the daemon MUST stay on loopback
	// unless the operator explicitly widens it.
	BindAddr string
	// Port is the TCP port the managed redis-server listens on.
	// Defaults to 6379 (the upstream default) but operators commonly
	// pick a non-default port like 6380 to avoid colliding with a
	// pre-existing system Redis. Pass 0 for the default.
	Port uint16
	// DataDir is the working directory for redis-server. Defaults to
	// the supervisord log directory so dump.rdb / appendonly.aof
	// stay co-located with the loco-server runtime. Redis is run
	// with `--save ""` and `--appendonly no` by default (see
	// `EphemeralPersistence`), so this dir mostly stays empty in
	// practice.
	DataDir string
	// EphemeralPersistence, when true (default), disables RDB
	// snapshots and AOF — state is cheap to rebuild from SQLite +
	// re-issued daemon pulls on next boot, so persisting it adds
	// latency for no upside.
	EphemeralPersistence bool
	// Disable, when true, removes redis-server from the managed
	// process set. Used when the operator runs an external Redis
	// (e.g. on another host) and points loco-server at it via
	// --redis-addr.
	Disable bool
}

func redisProgramSpec(cfg RedisConfig) ProgramSpec {
	bin := cfg.Bin
	if bin == "" {
		bin = "redis-server"
	}
	bind := cfg.BindAddr
	if bind == "" {
		bind = "127.0.0.1"
	}
	port := cfg.Port
	if port == 0 {
		port = 6379
	}
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = "."
	}

	persistArgs := ""
	if cfg.EphemeralPersistence {
		persistArgs = `--save "" --appendonly no `
	}
	cmd := fmt.Sprintf(
		`%s --bind %s --port %d --dir %s --daemonize no --protected-mode no --logfile "" %s`,
		bin, bind, port, dataDir, persistArgs,
	)

	return ProgramSpec{
		Name:         "redis",
		Command:      cmd,
		Autostart:    true,
		Autorestart:  true,
		StartSecs:    2,
		StopWaitSecs: 10,
	}
}
