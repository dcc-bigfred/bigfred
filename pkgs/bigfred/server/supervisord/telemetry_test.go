package supervisord

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAlloyProgramSpecUsesConfigPath(t *testing.T) {
	spec := alloyProgramSpec(TelemetryConfig{
		Enable:      true,
		ConfigPath:  "/data/etc/alloy.conf",
		StoragePath: "/data/alloy",
	})
	want := "alloy run --storage.path=/data/alloy /data/etc/alloy.conf"
	if spec.Command != want {
		t.Fatalf("command = %q, want %q", spec.Command, want)
	}
}

func TestAlloyProgramSpecDefaultConfigPath(t *testing.T) {
	spec := alloyProgramSpec(TelemetryConfig{
		Enable:      true,
		StoragePath: "/data/alloy",
	})
	want := "alloy run --storage.path=/data/alloy " + DefaultTelemetryConfigPath
	if spec.Command != want {
		t.Fatalf("command = %q, want %q", spec.Command, want)
	}
}

func TestPrepareAlloyTelemetryOverwritesConfigPath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "etc", "alloy.conf")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("// stale config"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := PrepareAlloyTelemetry(TelemetryConfig{ConfigPath: configPath}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "stale config") {
		t.Fatal("stale config was not overwritten")
	}
	if !strings.Contains(content, "otelcol.exporter.otlphttp") {
		t.Fatal("expected Grafana Cloud exporter in config")
	}
	if !strings.Contains(content, "otelcol.auth.basic") {
		t.Fatal("expected Grafana Cloud auth in config")
	}
	if !strings.Contains(content, `endpoint = "127.0.0.1:4317"`) {
		t.Fatal("missing default OTLP endpoint")
	}
}

func TestPrepareAlloyTelemetryCreatesParentDir(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing", "alloy.conf")
	if err := PrepareAlloyTelemetry(TelemetryConfig{ConfigPath: configPath}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareAlloyTelemetryCustomEndpoint(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "alloy.conf")
	if err := PrepareAlloyTelemetry(TelemetryConfig{ConfigPath: configPath, OTLPEndpoint: "127.0.0.1:9999"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `endpoint = "127.0.0.1:9999"`) {
		t.Fatal("custom endpoint not templated")
	}
}
