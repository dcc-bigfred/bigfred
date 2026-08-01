package supervisord

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseStatusOutput(t *testing.T) {
	rows := parseStatusOutput(`scripts-executor           RUNNING   pid 123, uptime 0:00:10
helper                     STOPPED   Not started
dcc-bus:dcc-bus-1-2        RUNNING   pid 456, uptime 0:01:00
`)
	if len(rows) != 3 {
		t.Fatalf("rows: %d", len(rows))
	}
	if rows[0].Name != "scripts-executor" || rows[0].Status != "RUNNING" || rows[0].PID != 123 {
		t.Fatalf("row0: %+v", rows[0])
	}
	if rows[1].Name != "helper" || rows[1].Status != "STOPPED" {
		t.Fatalf("row1: %+v", rows[1])
	}
	if rows[2].Name != "dcc-bus:dcc-bus-1-2" || rows[2].Status != "RUNNING" || rows[2].PID != 456 {
		t.Fatalf("row2: %+v", rows[2])
	}
}

func TestBareProgramName(t *testing.T) {
	if got := bareProgramName("dcc-bus:dcc-bus-1-2"); got != "dcc-bus-1-2" {
		t.Fatalf("got %q", got)
	}
	if got := bareProgramName("redis"); got != "redis" {
		t.Fatalf("got %q", got)
	}
}

// TestStatusAcceptsExitCode3 verifies that supervisorctl's documented
// exit code 3 (some processes not RUNNING) still yields parsed rows.
func TestStatusAcceptsExitCode3(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-supervisorctl")
	content := `#!/bin/sh
# Ignore -c <config>; emulate "status" with a stopped process.
shift 2
if [ "$1" = "status" ]; then
  printf '%s\n' 'dcc-bus:dcc-bus-1-2        STOPPED   Not started'
  printf '%s\n' 'redis                     RUNNING   pid 1, uptime 0:00:01'
  exit 3
fi
echo "unexpected: $*" >&2
exit 1
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	ctl := Ctl{Bin: script, ConfigPath: filepath.Join(dir, "supervisord.conf")}
	rows, err := ctl.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d %+v", len(rows), rows)
	}
	if rows[0].Name != "dcc-bus:dcc-bus-1-2" || rows[0].Status != "STOPPED" {
		t.Fatalf("row0: %+v", rows[0])
	}
	if rows[1].Name != "redis" || rows[1].Status != "RUNNING" {
		t.Fatalf("row1: %+v", rows[1])
	}
}

func TestStatusRejectsHardFailure(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-supervisorctl")
	content := `#!/bin/sh
shift 2
echo "unix:///tmp/x refused connection" >&2
exit 2
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	ctl := Ctl{Bin: script, ConfigPath: filepath.Join(dir, "supervisord.conf")}
	if _, err := ctl.Status(context.Background()); err == nil {
		t.Fatal("expected error on hard failure")
	}
}
