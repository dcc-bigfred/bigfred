package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestParse(t *testing.T) {
	text := `# comment
HTTP=0.0.0.0:9090
DB=/data/bigfred.db
JWT_SECRET=sekret
CORS_ORIGIN=https://hub.local,https://hub.local:443
SECURE_COOKIE=true
NO_SUPERVISOR=true
LOG_LEVEL=debug
REDIS_BIN=redis-server
REDIS_BIND=10.0.0.1
REDIS_PORT=6380
REDIS_DATA_DIR=/data/redis
REDIS_ADDR=10.0.0.1:6380
REDIS_EXTERNAL=true
REDIS_AUTO_DETECT=false
REDIS_RDB_SAVE=60:100,300:10
REDIS_NO_PERSIST=true
ENABLE_TELEMETRY=true
TELEMETRY_CONFIG=/custom/alloy.conf
REMOTE_ICMP_INTERVAL_SECS=15
REMOTE_ICMP_TARGETS_INTERVAL_SECS=8
`
	got := Parse(text)
	if got.HTTP != "0.0.0.0:9090" {
		t.Fatalf("HTTP = %q", got.HTTP)
	}
	if got.DB != "/data/bigfred.db" {
		t.Fatalf("DB = %q", got.DB)
	}
	if got.JWTSecret != "sekret" {
		t.Fatalf("JWTSecret = %q", got.JWTSecret)
	}
	if len(got.CorsOrigins) != 2 || got.CorsOrigins[0] != "https://hub.local" {
		t.Fatalf("CorsOrigins = %v", got.CorsOrigins)
	}
	if got.SecureCookie == nil || !*got.SecureCookie {
		t.Fatal("expected SecureCookie true")
	}
	if got.NoSupervisor == nil || !*got.NoSupervisor {
		t.Fatal("expected NoSupervisor true")
	}
	if got.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q", got.LogLevel)
	}
	if got.RedisBin != "redis-server" {
		t.Fatalf("RedisBin = %q", got.RedisBin)
	}
	if got.RedisBindAddr != "10.0.0.1" {
		t.Fatalf("RedisBindAddr = %q", got.RedisBindAddr)
	}
	if got.RedisPort == nil || *got.RedisPort != 6380 {
		t.Fatalf("RedisPort = %v", got.RedisPort)
	}
	if got.RedisDataDir != "/data/redis" {
		t.Fatalf("RedisDataDir = %q", got.RedisDataDir)
	}
	if got.RedisAddr != "10.0.0.1:6380" {
		t.Fatalf("RedisAddr = %q", got.RedisAddr)
	}
	if got.RedisExternal == nil || !*got.RedisExternal {
		t.Fatal("expected RedisExternal true")
	}
	if got.RedisAutoDetect == nil || *got.RedisAutoDetect {
		t.Fatal("expected RedisAutoDetect false")
	}
	if len(got.RedisRDBSave) != 2 || got.RedisRDBSave[0] != "60:100" {
		t.Fatalf("RedisRDBSave = %v", got.RedisRDBSave)
	}
	if got.RedisNoPersist == nil || !*got.RedisNoPersist {
		t.Fatal("expected RedisNoPersist true")
	}
	if got.EnableTelemetry == nil || !*got.EnableTelemetry {
		t.Fatal("expected EnableTelemetry true")
	}
	if got.TelemetryConfig != "/custom/alloy.conf" {
		t.Fatalf("TelemetryConfig = %q", got.TelemetryConfig)
	}
	if got.RemoteICMPIntervalSecs == nil || *got.RemoteICMPIntervalSecs != 15 {
		t.Fatalf("RemoteICMPIntervalSecs = %v", got.RemoteICMPIntervalSecs)
	}
	if got.RemoteICMPTargetsIntervalSecs == nil || *got.RemoteICMPTargetsIntervalSecs != 8 {
		t.Fatalf("RemoteICMPTargetsIntervalSecs = %v", got.RemoteICMPTargetsIntervalSecs)
	}
}

func TestLoadOrCreateCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loco-server.conf")

	cfg, err := LoadOrCreate(path, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP != "0.0.0.0:8080" {
		t.Fatalf("HTTP = %q", cfg.HTTP)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file created: %v", err)
	}

	if err := os.WriteFile(path, []byte("HTTP=127.0.0.1:3000\nLOG_LEVEL=warn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadOrCreate(path, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP != "127.0.0.1:3000" || cfg.LogLevel != "warn" {
		t.Fatalf("got HTTP=%q LogLevel=%q", cfg.HTTP, cfg.LogLevel)
	}
}

func TestWriteDefaultsReference(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loco-server.conf.defaults")

	if err := WriteDefaultsReference(path, DefaultFile()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, key := range []string{
		"JWT_SECRET=", "REDIS_DATA_DIR=", "REDIS_ADDR=", "REDIS_RDB_SAVE=",
		"ENABLE_TELEMETRY=", "TELEMETRY_CONFIG=", "REMOTE_ICMP_INTERVAL_SECS=",
		"REMOTE_ICMP_TARGETS_INTERVAL_SECS=", "reference only",
	} {
		if !strings.Contains(text, key) {
			t.Fatalf("defaults reference missing %q in:\n%s", key, text)
		}
	}
}

func TestLoadOrCreateMissingDirWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "subdir", "loco-server.conf")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	cfg, err := LoadOrCreate(path, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP != "0.0.0.0:8080" {
		t.Fatalf("expected built-in defaults, got HTTP=%q", cfg.HTTP)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no file at %s", path)
	}
}
