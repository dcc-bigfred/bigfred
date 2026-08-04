package microinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedisServiceDefCreatesDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "var", "lib", "redis")
	svc, err := RedisServiceDef(RedisConfig{
		Bin:     "valkey-server",
		DataDir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("expected data dir %s to exist: %v", dir, err)
	}
	if !strings.Contains(svc.StartCmd, "--dir '"+dir+"'") && !strings.Contains(svc.StartCmd, "--dir "+dir) {
		// shellQuote wraps with single quotes
		if !strings.Contains(svc.StartCmd, dir) {
			t.Fatalf("StartCmd missing data dir: %s", svc.StartCmd)
		}
	}
	if svc.Name != "redis" {
		t.Fatalf("Name = %q", svc.Name)
	}
}

func TestRedisServiceDefDefaultDataDir(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", t.TempDir())
	t.Setenv("DATA_DIR", "")
	svc, err := RedisServiceDef(RedisConfig{Bin: "redis-server"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svc.StartCmd, filepath.Join("var", "lib", "redis")) {
		t.Fatalf("expected default var/lib/redis in StartCmd: %s", svc.StartCmd)
	}
}
