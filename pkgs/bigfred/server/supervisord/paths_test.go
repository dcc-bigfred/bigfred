package supervisord

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

func TestDefaultPathsHubLayout(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", "")
	t.Setenv("DATA_DIR", "")

	p, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if p.ConfigDir != datadir.Path("etc", "supervisord") {
		t.Fatalf("ConfigDir: %q", p.ConfigDir)
	}
	if p.ConfigPath != datadir.Path("etc", "supervisord", "supervisord.conf") {
		t.Fatalf("ConfigPath: %q", p.ConfigPath)
	}
	if p.InetHTTPAddr != "127.0.0.1:9001" {
		t.Fatalf("InetHTTPAddr: %q", p.InetHTTPAddr)
	}
	if p.PIDFile != datadir.Path("run", "supervisord.pid") {
		t.Fatalf("PIDFile: %q", p.PIDFile)
	}
	if p.LogDir != datadir.Path("logs") {
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
	if p.ConfigDir != datadir.Path("etc", "supervisord") {
		t.Fatalf("ConfigDir: %q", p.ConfigDir)
	}
	if p.LogDir != datadir.Path("logs") {
		t.Fatalf("LogDir: %q", p.LogDir)
	}
	if p.InetHTTPAddr != "127.0.0.1:9001" {
		t.Fatalf("InetHTTPAddr: %q", p.InetHTTPAddr)
	}
}
