package supervisord

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAlloyProgramSpecUsesSingleConfigPath(t *testing.T) {
	spec := alloyProgramSpec(TelemetryConfig{
		Enable:               true,
		ConfigPath:           "/data/etc/alloy.conf",
		StoragePath:          "/data/alloy",
		SupervisordConfigDir: "/run/bigfred",
	})
	want := "alloy run --storage.path=/data/alloy /run/bigfred/alloy"
	if spec.Command != want {
		t.Fatalf("command = %q, want %q", spec.Command, want)
	}
}

func TestAlloyProgramSpecConfigDirectory(t *testing.T) {
	dir := t.TempDir()
	spec := alloyProgramSpec(TelemetryConfig{
		Enable:               true,
		ConfigPath:           dir,
		StoragePath:          "/data/alloy",
		SupervisordConfigDir: "/run/bigfred",
	})
	want := "alloy run --storage.path=/data/alloy " + dir
	if spec.Command != want {
		t.Fatalf("command = %q, want %q", spec.Command, want)
	}
}

func TestAlloyProgramSpecWithoutTelemetry(t *testing.T) {
	spec := alloyProgramSpec(TelemetryConfig{
		ConfigPath:  "/data/etc/alloy.conf",
		StoragePath: "/data/alloy",
	})
	want := "alloy run --storage.path=/data/alloy /data/etc/alloy.conf"
	if spec.Command != want {
		t.Fatalf("command = %q, want %q", spec.Command, want)
	}
}

func clearGrafanaCloudOTELenv(t *testing.T) {
	t.Helper()
	t.Setenv("GRAFANA_CLOUD_OTLP_ENDPOINT", "")
	t.Setenv("GRAFANA_CLOUD_INSTANCE_ID", "")
	t.Setenv("GRAFANA_CLOUD_API_KEY", "")
}

func TestPrepareAlloyTelemetryLinksOperatorConfig(t *testing.T) {
	clearGrafanaCloudOTELenv(t)
	root := t.TempDir()
	operator := filepath.Join(root, "operator.conf")
	if err := os.WriteFile(operator, []byte("// operator"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := TelemetryConfig{ConfigPath: operator}
	if err := PrepareAlloyTelemetry(root, cfg); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(alloyRuntimeDir(root), operatorAlloyLinkName)
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if target != operator {
		t.Fatalf("link target = %q, want %q", target, operator)
	}
	data, err := os.ReadFile(filepath.Join(alloyRuntimeDir(root), bigfredOTELFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `endpoint = "127.0.0.1:4317"`) {
		t.Fatal("missing default OTLP endpoint")
	}
}

func TestPrepareAlloyTelemetryWritesIntoConfigDirectory(t *testing.T) {
	clearGrafanaCloudOTELenv(t)
	dir := t.TempDir()
	if err := PrepareAlloyTelemetry(t.TempDir(), TelemetryConfig{ConfigPath: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, bigfredOTELFilename)); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareAlloyTelemetryCustomEndpoint(t *testing.T) {
	clearGrafanaCloudOTELenv(t)
	root := t.TempDir()
	if err := PrepareAlloyTelemetry(root, TelemetryConfig{OTLPEndpoint: "127.0.0.1:9999"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(alloyRuntimeDir(root), bigfredOTELFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `endpoint = "127.0.0.1:9999"`) {
		t.Fatal("custom endpoint not templated")
	}
	if strings.Contains(string(data), "otelcol.exporter.otlphttp") {
		t.Fatal("expected local-only config without Grafana Cloud exporter")
	}
}

func TestPrepareAlloyTelemetryGrafanaCloudMode(t *testing.T) {
	t.Setenv("GRAFANA_CLOUD_OTLP_ENDPOINT", "https://otlp.example.com/otlp")
	t.Setenv("GRAFANA_CLOUD_INSTANCE_ID", "123456")
	t.Setenv("GRAFANA_CLOUD_API_KEY", "glc_test")

	root := t.TempDir()
	if err := PrepareAlloyTelemetry(root, TelemetryConfig{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(alloyRuntimeDir(root), bigfredOTELFilename))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "otelcol.exporter.otlphttp") {
		t.Fatal("expected Grafana Cloud exporter in config")
	}
	if !strings.Contains(content, "otelcol.auth.basic") {
		t.Fatal("expected Grafana Cloud auth in config")
	}
}
