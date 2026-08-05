package otelenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseIgnoresCommentsAndBlanks(t *testing.T) {
	got := Parse(`
# comment
ENABLE_TELEMETRY=true

OTEL_EXPORTER_OTLP_ENDPOINT="127.0.0.1:4317"
invalid-line
OTEL_SERVICE_NAME=microinit
`)
	if got["ENABLE_TELEMETRY"] != "true" {
		t.Fatalf("ENABLE_TELEMETRY=%q", got["ENABLE_TELEMETRY"])
	}
	if got["OTEL_EXPORTER_OTLP_ENDPOINT"] != "127.0.0.1:4317" {
		t.Fatalf("endpoint=%q", got["OTEL_EXPORTER_OTLP_ENDPOINT"])
	}
	if got["OTEL_SERVICE_NAME"] != "microinit" {
		t.Fatalf("service=%q", got["OTEL_SERVICE_NAME"])
	}
}

func TestLoadMissingIsNoop(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDoesNotOverrideExistingEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otel.env")
	if err := os.WriteFile(path, []byte("OTEL_TEST_KEY=fromfile\nOTEL_TEST_ONLY_FILE=yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OTEL_TEST_KEY", "fromenv")
	_ = os.Unsetenv("OTEL_TEST_ONLY_FILE")

	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("OTEL_TEST_KEY"); got != "fromenv" {
		t.Fatalf("OTEL_TEST_KEY=%q, want fromenv", got)
	}
	if got := os.Getenv("OTEL_TEST_ONLY_FILE"); got != "yes" {
		t.Fatalf("OTEL_TEST_ONLY_FILE=%q, want yes", got)
	}
}

func TestEnvBool(t *testing.T) {
	t.Setenv("OTEL_BOOL_TRUE", "true")
	t.Setenv("OTEL_BOOL_FALSE", "0")
	if v, set := EnvBool("OTEL_BOOL_TRUE"); !set || !v {
		t.Fatalf("true: v=%v set=%v", v, set)
	}
	if v, set := EnvBool("OTEL_BOOL_FALSE"); !set || v {
		t.Fatalf("false: v=%v set=%v", v, set)
	}
	if _, set := EnvBool("OTEL_BOOL_MISSING"); set {
		t.Fatal("missing should be unset")
	}
}

func TestWriteDefaultsReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "etc", "otel.env.defaults")
	if err := WriteDefaultsReference(path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || !strings.Contains(string(raw), "ENABLE_TELEMETRY") {
		t.Fatalf("unexpected content: %s", raw)
	}
}
