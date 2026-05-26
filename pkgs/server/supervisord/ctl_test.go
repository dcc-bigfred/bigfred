package supervisord

import "testing"

func TestParseStatusOutput(t *testing.T) {
	rows := parseStatusOutput(`scripts-executor           RUNNING   pid 123, uptime 0:00:10
helper                     STOPPED   Not started
`)
	if len(rows) != 2 {
		t.Fatalf("rows: %d", len(rows))
	}
	if rows[0].Name != "scripts-executor" || rows[0].Status != "RUNNING" || rows[0].PID != 123 {
		t.Fatalf("row0: %+v", rows[0])
	}
	if rows[1].Name != "helper" || rows[1].Status != "STOPPED" {
		t.Fatalf("row1: %+v", rows[1])
	}
}
