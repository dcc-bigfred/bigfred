package microinit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedisServiceDefCreatesDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "var", "lib", "redis")
	svc, err := RedisServiceDef(RedisConfig{
		Bin:     "valkey-server",
		DataDir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("expected data dir %s to exist: %v", dir, err)
	}
	if !strings.Contains(svc.StartCmd, "--dir '"+dir+"'") && !strings.Contains(svc.StartCmd, "--dir "+dir) {
		// shellQuote wraps with single quotes
		if !strings.Contains(svc.StartCmd, dir) {
			t.Fatalf("StartCmd missing data dir: %s", svc.StartCmd)
		}
	}
	if svc.Name != "redis" {
		t.Fatalf("Name = %q", svc.Name)
	}
	if svc.Labels[LabelCreatedBy] != CreatedByBigfred {
		t.Fatalf("labels: %+v", svc.Labels)
	}
	if svc.RestartPolicy != RestartAlways {
		t.Fatalf("RestartPolicy = %q, want %q", svc.RestartPolicy, RestartAlways)
	}
}

func TestRedisServiceDefDefaultDataDir(t *testing.T) {
	t.Setenv("BIGFRED_DATA_DIR", t.TempDir())
	t.Setenv("DATA_DIR", "")
	svc, err := RedisServiceDef(RedisConfig{Bin: "redis-server"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svc.StartCmd, filepath.Join("var", "db", "redis")) {
		t.Fatalf("expected default var/db/redis in StartCmd: %s", svc.StartCmd)
	}
}

func TestPrepareAlloyTelemetryRendersValidBlocks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BIGFRED_DATA_DIR", root)
	t.Setenv("DATA_DIR", "")
	path := filepath.Join(root, "etc", "alloy.conf")
	if err := PrepareAlloyTelemetry(TelemetryConfig{
		ConfigPath:   path,
		OTLPEndpoint: "127.0.0.1:4317",
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		`otelcol.receiver.otlp "bigfred" {`,
		`endpoint = "127.0.0.1:4317"`,
		`endpoint = "127.0.0.1:4318"`,
		`otelcol.processor.batch "bigfred" {`,
		`otelcol.exporter.otlphttp "bigfred" {`,
		`otelcol.auth.basic "bigfred" {`,
		`username = sys.env("GRAFANA_CLOUD_INSTANCE_ID")`,
		`password = sys.env("GRAFANA_CLOUD_API_KEY")`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("alloy.conf missing %q; got:\n%s", want, s)
		}
	}
	// Nested blocks must not be collapsed onto one line (River parse failure).
	if strings.Contains(s, "grpc { endpoint") {
		t.Fatalf("alloy.conf still has compacted nested blocks:\n%s", s)
	}
	if strings.Contains(s, "http { endpoint") {
		t.Fatalf("alloy.conf still has compacted nested http blocks:\n%s", s)
	}
}
