package supervisord

import "testing"

func TestDefaultPathsHubLayout(t *testing.T) {
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
	if p.SocketPath != "/data/run/supervisord.sock" {
		t.Fatalf("SocketPath: %q", p.SocketPath)
	}
	if p.PIDFile != "/data/run/supervisord.pid" {
		t.Fatalf("PIDFile: %q", p.PIDFile)
	}
	if p.LogDir != "/data/logs" {
		t.Fatalf("LogDir: %q", p.LogDir)
	}
}
