// Package config loads dotenv-style settings for loco-server from
// /data/etc/loco-server.conf on hub images. CLI flags override file values.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// DefaultPath is the persistent configuration file on hub images.
const DefaultPath = "/data/etc/loco-server.conf"

// DefaultTemplatePath is rewritten on every loco-server start with built-in
// defaults and documents every supported KEY. It is not read at runtime.
const DefaultTemplatePath = "/data/etc/loco-server.conf.defaults"

// File holds settings from a KEY=value configuration file.
type File struct {
	HTTP            string
	DB              string
	JWTSecret       string
	CorsOrigins     []string
	SecureCookie    *bool
	NoSupervisor    *bool
	LogLevel        string
	RedisBin        string
	RedisBindAddr   string
	RedisPort       *uint16
	RedisDataDir    string
	RedisAddr       string
	RedisExternal   *bool
	RedisAutoDetect *bool
	RedisRDBSave    []string
	RedisNoPersist  *bool
	EnableTelemetry *bool
	TelemetryConfig string
}

// DefaultFile returns built-in defaults (mirrors loco-server CLI flag defaults).
func DefaultFile() File {
	port := uint16(6379)
	return File{
		HTTP:            "0.0.0.0:8080",
		DB:              "bigfred.db",
		CorsOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		LogLevel:        "info",
		RedisBin:        "valkey-server",
		RedisBindAddr:   "127.0.0.1",
		RedisPort:       &port,
		TelemetryConfig: service.DefaultTelemetryConfigPath,
	}
}

// LoadOrCreate reads path when present. When the file is missing it is
// created with DefaultFile(); a warning is logged when creation fails.
// DefaultTemplatePath is always rewritten with built-in defaults (reference only).
func LoadOrCreate(path string, log *logrus.Logger) (*File, error) {
	if log == nil {
		log = logrus.New()
	}
	defaults := DefaultFile()
	if writeErr := WriteDefaultsReference(DefaultTemplatePath, defaults); writeErr != nil {
		log.WithError(writeErr).Warnf("cannot write %s", DefaultTemplatePath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if writeErr := Write(path, defaults); writeErr != nil {
			log.WithError(writeErr).Warnf("cannot create %s; using built-in defaults", path)
		} else {
			log.WithField("path", path).Info("created default configuration file")
		}
		return &defaults, nil
	}
	f := Parse(string(data))
	return &f, nil
}

// Parse reads KEY=value lines (dotenv). Comments (#) and blank lines are ignored.
func Parse(text string) File {
	f := File{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToUpper(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		switch key {
		case "HTTP", "HTTP_ADDR", "LISTEN":
			f.HTTP = value
		case "DB", "DB_PATH":
			f.DB = value
		case "JWT_SECRET", "JWTSECRET":
			f.JWTSecret = value
		case "CORS_ORIGIN", "CORS_ORIGINS", "CORS-ORIGIN":
			if value != "" {
				f.CorsOrigins = splitList(value)
			}
		case "SECURE_COOKIE", "SECURECOOKIE":
			v := parseBool(value)
			f.SecureCookie = &v
		case "NO_SUPERVISOR", "NOSUPERVISOR":
			v := parseBool(value)
			f.NoSupervisor = &v
		case "LOG_LEVEL", "LOGLEVEL":
			f.LogLevel = value
		case "REDIS_BIN", "REDISBIN":
			f.RedisBin = value
		case "REDIS_BIND", "REDIS_BIND_ADDR", "REDISBIND":
			f.RedisBindAddr = value
		case "REDIS_PORT", "REDISPORT":
			if n, err := strconv.ParseUint(value, 10, 16); err == nil {
				p := uint16(n)
				f.RedisPort = &p
			}
		case "REDIS_DATA_DIR", "REDISDATADIR":
			f.RedisDataDir = value
		case "REDIS_ADDR", "REDISADDR":
			f.RedisAddr = value
		case "REDIS_EXTERNAL", "REDISEXTERNAL":
			v := parseBool(value)
			f.RedisExternal = &v
		case "REDIS_AUTO_DETECT", "REDISAUTODETECT":
			v := parseBool(value)
			f.RedisAutoDetect = &v
		case "REDIS_RDB_SAVE", "REDISRDBSAVE":
			if value == "" {
				f.RedisRDBSave = []string{}
			} else {
				f.RedisRDBSave = splitList(value)
			}
		case "REDIS_NO_PERSIST", "REDISNOPERSIST":
			v := parseBool(value)
			f.RedisNoPersist = &v
		case "ENABLE_TELEMETRY", "ENABLETELEMETRY":
			v := parseBool(value)
			f.EnableTelemetry = &v
		case "TELEMETRY_CONFIG", "TELEMETRYCONFIG":
			f.TelemetryConfig = value
		}
	}
	return f
}

// Write creates path with defaults rendered as a commented dotenv file.
func Write(path string, defaults File) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := renderConfig(defaults)
	return os.WriteFile(path, []byte(content), 0o644)
}

// WriteDefaultsReference writes the built-in defaults reference template.
// The file is not read at runtime; copy keys into loco-server.conf to apply them.
func WriteDefaultsReference(path string, defaults File) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := renderDefaultsReference(defaults)
	return os.WriteFile(path, []byte(content), 0o644)
}

func renderConfig(d File) string {
	cors := strings.Join(d.CorsOrigins, ",")
	redisPort := uint16(6379)
	if d.RedisPort != nil {
		redisPort = *d.RedisPort
	}
	return fmt.Sprintf(`# loco-server — BigFred web backend (REST + WebSocket)
# Edit on the device under /data/etc/ (persists across image updates).
# CLI flags override values in this file.

HTTP=%s
DB=%s
# JWT_SECRET=          # empty = BIGFRED_JWT_SECRET env or random per-run secret
CORS_ORIGIN=%s
SECURE_COOKIE=false
NO_SUPERVISOR=false
LOG_LEVEL=%s

REDIS_BIN=%s
REDIS_BIND=%s
REDIS_PORT=%d
# REDIS_DATA_DIR=      # defaults to supervisord config directory
# REDIS_ADDR=          # defaults to REDIS_BIND:REDIS_PORT
REDIS_EXTERNAL=false
REDIS_AUTO_DETECT=true
# REDIS_RDB_SAVE=60:100 # comma-separated seconds:changes pairs; empty disables
REDIS_NO_PERSIST=false

ENABLE_TELEMETRY=false
TELEMETRY_CONFIG=%s
`, d.HTTP, d.DB, cors, d.LogLevel, d.RedisBin, d.RedisBindAddr, redisPort, d.TelemetryConfig)
}

func renderDefaultsReference(d File) string {
	cors := strings.Join(d.CorsOrigins, ",")
	redisPort := uint16(6379)
	if d.RedisPort != nil {
		redisPort = *d.RedisPort
	}
	return fmt.Sprintf(`# loco-server.conf.defaults — built-in defaults (reference only)
# Auto-generated on every loco-server start. Not read at runtime.
# Copy keys into %s to persist changes. CLI flags override the live config.

# HTTP listen address (flag: --http)
HTTP=%s

# SQLite database path (flag: --db)
DB=%s

# JWT signing secret; empty uses BIGFRED_JWT_SECRET or a random per-run secret (flag: --jwt-secret)
JWT_SECRET=

# Comma-separated CORS allowed origins (flag: --cors-origin)
CORS_ORIGIN=%s

# Set Secure on the session cookie; use true in production over HTTPS (flag: --secure-cookie)
SECURE_COOKIE=false

# Skip supervisord process management (flag: --no-supervisor)
NO_SUPERVISOR=false

# logrus level: debug, info, warn, error; BIGFRED_LOG_LEVEL env overrides (flag: --log-level)
LOG_LEVEL=%s

# redis-server / valkey-server binary for the managed daemon (flag: --redis-bin)
REDIS_BIN=%s

# Interface the managed redis-server binds on (flag: --redis-bind)
REDIS_BIND=%s

# TCP port for the managed redis-server (flag: --redis-port)
REDIS_PORT=%d

# Working directory for redis-server; empty = supervisord config directory (flag: --redis-data-dir)
REDIS_DATA_DIR=

# Redis dial address for loco-server and dcc-bus; empty = REDIS_BIND:REDIS_PORT (flag: --redis-addr)
REDIS_ADDR=

# Do not spawn managed redis-server; dial REDIS_ADDR instead (flag: --redis-external)
REDIS_EXTERNAL=false

# Reuse existing Redis at REDIS_ADDR without spawning (flag: --redis-auto-detect)
REDIS_AUTO_DETECT=true

# RDB snapshot rules as comma-separated seconds:changes pairs; empty disables (flag: --redis-rdb-save)
REDIS_RDB_SAVE=60:100

# Disable RDB snapshots for managed redis-server (flag: --redis-no-persist)
REDIS_NO_PERSIST=false

# Start Grafana Alloy via supervisord and enable dcc-bus metrics (flag: --enable-telemetry)
ENABLE_TELEMETRY=false

# Alloy config file path (flag: --telemetry-config)
TELEMETRY_CONFIG=%s
`, DefaultPath, d.HTTP, d.DB, cors, d.LogLevel, d.RedisBin, d.RedisBindAddr, redisPort, d.TelemetryConfig)
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
