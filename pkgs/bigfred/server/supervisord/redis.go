package supervisord

import (
	"fmt"
	"strconv"
	"strings"
)

// RDBSavePoint is one Redis "save <seconds> <changes>" snapshot rule.
// See https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/
type RDBSavePoint struct {
	Seconds int // minimum seconds since the last save
	Changes int // minimum dataset changes in that window
}

// DefaultRDBSavePoints is the managed redis-server RDB policy when the
// operator does not pass --redis-rdb-save or --redis-no-persist.
var DefaultRDBSavePoints = []RDBSavePoint{{Seconds: 60, Changes: 100}}

// RedisConfig collects the few knobs InfraProcesses exposes for the
// managed redis-server child. Redis is mandatory in BigFred (state
// cache + cross-process pub/sub between loco-server and dcc-bus); the
// managed instance is skipped when the operator passes --redis-external,
// or when --redis-auto-detect finds an existing server at --redis-addr
// (both set RedisConfig.Disable).
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
	// Defaults to 6379 (the upstream default)
	Port uint16
	// DataDir is the working directory for redis-server. Defaults to
	// the supervisord log directory so dump.rdb stays co-located with
	// the loco-server runtime.
	DataDir string
	// RDBSavePoints configures RDB snapshotting. Nil or empty disables
	// persistence (--save "" --appendonly no). When set, AOF is always
	// disabled and only the listed save rules are applied.
	RDBSavePoints []RDBSavePoint
	// Disable, when true, removes redis-server from the managed
	// process set. Used when the operator runs an external Redis
	// (e.g. on another host) and points loco-server at it via
	// --redis-addr.
	Disable bool
}

// ParseRDBSavePoint parses "seconds:changes", e.g. "60:100".
func ParseRDBSavePoint(value string) (RDBSavePoint, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return RDBSavePoint{}, fmt.Errorf("supervisord: redis rdb save %q: want seconds:changes", value)
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return RDBSavePoint{}, fmt.Errorf("supervisord: redis rdb save %q: invalid seconds: %w", value, err)
	}
	changes, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return RDBSavePoint{}, fmt.Errorf("supervisord: redis rdb save %q: invalid changes: %w", value, err)
	}
	if seconds <= 0 || changes <= 0 {
		return RDBSavePoint{}, fmt.Errorf("supervisord: redis rdb save %q: seconds and changes must be > 0", value)
	}
	return RDBSavePoint{Seconds: seconds, Changes: changes}, nil
}

// ParseRDBSavePoints parses repeatable CLI values like "60:100".
// Empty strings are ignored.
func ParseRDBSavePoints(values []string) ([]RDBSavePoint, error) {
	points := make([]RDBSavePoint, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		point, err := ParseRDBSavePoint(value)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
}

// ResolveRDBSavePoints applies loco-server CLI semantics for managed Redis.
// --redis-no-persist and --redis-rdb-save="" disable persistence; omitted
// flags use DefaultRDBSavePoints.
func ResolveRDBSavePoints(noPersist bool, flagValues []string) ([]RDBSavePoint, error) {
	if noPersist {
		return nil, nil
	}
	if len(flagValues) == 1 && strings.TrimSpace(flagValues[0]) == "" {
		return nil, nil
	}
	if len(flagValues) == 0 {
		return append([]RDBSavePoint(nil), DefaultRDBSavePoints...), nil
	}
	points, err := ParseRDBSavePoints(flagValues)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 {
		return nil, nil
	}
	return points, nil
}

func redisPersistenceArgs(points []RDBSavePoint) string {
	if len(points) == 0 {
		return `--save "" --appendonly no `
	}
	var b strings.Builder
	b.WriteString(`--appendonly no --save "" `)
	for _, point := range points {
		fmt.Fprintf(&b, "--save %d %d ", point.Seconds, point.Changes)
	}
	return b.String()
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

	cmd := fmt.Sprintf(
		`%s --bind %s --port %d --dir %s --daemonize no --protected-mode no --logfile "" %s`,
		bin, bind, port, dataDir, redisPersistenceArgs(cfg.RDBSavePoints),
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
