package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsService_resolveFileID(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "supervisord.conf")
	svc := &DiagnosticsService{configPath: cfg, logDir: dir}

	tests := []struct {
		id      string
		wantAbs string
		wantErr error
	}{
		{"supervisord.log", filepath.Join(dir, "supervisord.log"), nil},
		{"supervisord.config", cfg, nil},
		{"redis.stdout", filepath.Join(dir, "redis.stdout.log"), nil},
		{"alloy.stdout", filepath.Join(dir, "alloy.stdout.log"), nil},
		{"alloy.stderr", filepath.Join(dir, "alloy.stderr.log"), nil},
		{"dcc-bus.evil/../supervisord.log", "", ErrDiagnosticsForbidden},
		{"dcc-bus.not-a-bus.log", "", ErrDiagnosticsForbidden},
		{"unknown", "", ErrDiagnosticsForbidden},
	}
	for _, tc := range tests {
		abs, _, err := svc.resolveFileID(tc.id)
		if tc.wantErr != nil {
			if err != tc.wantErr {
				t.Fatalf("resolveFileID(%q) err=%v want %v", tc.id, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Fatalf("resolveFileID(%q): %v", tc.id, err)
		}
		if abs != tc.wantAbs {
			t.Fatalf("resolveFileID(%q) = %q want %q", tc.id, abs, tc.wantAbs)
		}
	}
}

func TestDiagnosticsService_Read(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "supervisord.log")
	if err := os.WriteFile(logPath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := &DiagnosticsService{configPath: filepath.Join(dir, "supervisord.conf"), logDir: dir}

	got, err := svc.Read("supervisord.log", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "line2") || !strings.Contains(got.Content, "line3") {
		t.Fatalf("content=%q", got.Content)
	}
}

func TestDiagnosticsService_listDccBusEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"dcc-bus-1-2.stdout.log",
		"dcc-bus-1-2.stderr.log",
		"redis.stdout.log",
		"supervisord.log",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	svc := &DiagnosticsService{configPath: filepath.Join(dir, "supervisord.conf"), logDir: dir}
	entries, err := svc.listDccBusEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries=%d want 2", len(entries))
	}
}

func TestDiagnosticsService_Sources_includesAlloy(t *testing.T) {
	dir := t.TempDir()
	svc := &DiagnosticsService{configPath: filepath.Join(dir, "supervisord.conf"), logDir: dir}
	src, err := svc.Sources()
	if err != nil {
		t.Fatal(err)
	}
	var alloy *DiagnosticGroup
	for i := range src.Groups {
		if src.Groups[i].ID == "alloy" {
			alloy = &src.Groups[i]
			break
		}
	}
	if alloy == nil {
		t.Fatal("alloy group missing")
	}
	if len(alloy.Entries) != 2 || alloy.Entries[0].ID != "alloy.stdout" {
		t.Fatalf("alloy entries: %+v", alloy.Entries)
	}
}
