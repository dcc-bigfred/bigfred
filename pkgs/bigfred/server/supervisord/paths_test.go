package supervisord

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathsHubLayout(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", "")
	t.Setenv("DATA_DIR", "")

	p, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if p.ConfigDir != "/data/etc/supervisord" {
		t.Fatalf("ConfigDir: %q", p.ConfigDir)
	}
	if p.ConfigPath != "/data/etc/supervisord/supervisord.conf" {
		t.Fatalf("ConfigPath: %q", p.ConfigPath)
	}
	if p.InetHTTPAddr != "127.0.0.1:9001" {
		t.Fatalf("InetHTTPAddr: %q", p.InetHTTPAddr)
	}
	if p.PIDFile != "/data/run/supervisord.pid" {
		t.Fatalf("PIDFile: %q", p.PIDFile)
	}
	if p.LogDir != "/data/logs" {
		t.Fatalf("LogDir: %q", p.LogDir)
	}
}

func TestDefaultPathsRespectsDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", dir)

	p, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if p.ConfigDir != filepath.Join(dir, "etc", "supervisord") {
		t.Fatalf("ConfigDir: %q", p.ConfigDir)
	}
	if p.LogDir != filepath.Join(dir, "logs") {
		t.Fatalf("LogDir: %q", p.LogDir)
	}
	if p.InetHTTPAddr != "127.0.0.1:9001" {
		t.Fatalf("InetHTTPAddr: %q", p.InetHTTPAddr)
	}
}
