package cli

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/cli/config"
)

func TestApplyConfigRespectsCLIOverrides(t *testing.T) {
	f := Flags{
		HTTPAddr: "0.0.0.0:8080",
		DBPath:   "bigfred.db",
		LogLevel: "info",
	}
	cfg := &config.File{
		HTTP:     "0.0.0.0:9090",
		DB:       "/data/bigfred.db",
		LogLevel: "debug",
	}
	changed := func(name string) bool {
		return name == "http"
	}
	applyConfig(&f, cfg, changed)
	if f.HTTPAddr != "0.0.0.0:8080" {
		t.Fatalf("CLI http override failed: %q", f.HTTPAddr)
	}
	if f.DBPath != "/data/bigfred.db" {
		t.Fatalf("expected file db: %q", f.DBPath)
	}
	if f.LogLevel != "debug" {
		t.Fatalf("expected file log-level: %q", f.LogLevel)
	}
}

func TestApplyConfigAppliesFileValues(t *testing.T) {
	f := Flags{}
	secure := true
	telemetry := true
	cfg := &config.File{
		HTTP:            "192.168.1.10:8080",
		SecureCookie:    &secure,
		CorsOrigins:     []string{"https://example.com"},
		EnableTelemetry: &telemetry,
		TelemetryConfig: "/data/etc/custom-alloy.conf",
	}
	applyConfig(&f, cfg, func(string) bool { return false })
	if f.HTTPAddr != "192.168.1.10:8080" {
		t.Fatalf("HTTPAddr = %q", f.HTTPAddr)
	}
	if !f.SecureCookie {
		t.Fatal("expected secure cookie from file")
	}
	if len(f.AllowedOrigins) != 1 || f.AllowedOrigins[0] != "https://example.com" {
		t.Fatalf("AllowedOrigins = %v", f.AllowedOrigins)
	}
	if !f.EnableTelemetry {
		t.Fatal("expected enable-telemetry from file")
	}
	if f.TelemetryConfig != "/data/etc/custom-alloy.conf" {
		t.Fatalf("TelemetryConfig = %q", f.TelemetryConfig)
	}
}
