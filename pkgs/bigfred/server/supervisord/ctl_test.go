package supervisord

import "testing"

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
