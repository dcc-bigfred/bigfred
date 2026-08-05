package cli

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestRootCommandParsesEnableTelemetry(t *testing.T) {
	cmd := NewRootCommand(logrus.New())
	cmd.SetArgs([]string{"--enable-telemetry"})
	if err := cmd.ParseFlags([]string{"--enable-telemetry"}); err != nil {
		t.Fatal(err)
	}
	got, err := cmd.Flags().GetBool("enable-telemetry")
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected --enable-telemetry to parse as true")
	}
}

func TestRootCommandIgnoresFlagsAfterBareDoubleDash(t *testing.T) {
	cmd := NewRootCommand(logrus.New())
	// go run ./pkgs/bigfred/server -- --enable-telemetry passes a literal "--"
	// as argv[1], which makes cobra treat following tokens as positional args.
	cmd.SetArgs([]string{"--", "--enable-telemetry"})
	if err := cmd.ParseFlags([]string{"--", "--enable-telemetry"}); err != nil {
		t.Fatal(err)
	}
	got, err := cmd.Flags().GetBool("enable-telemetry")
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("expected flag to stay false when a bare -- precedes it")
	}
}

func TestApplyOtelEnvFlagsOverridesConf(t *testing.T) {
	cmd := NewRootCommand(logrus.New())
	f := Flags{EnableTelemetry: false}
	t.Setenv("ENABLE_TELEMETRY", "true")
	applyOtelEnvFlags(cmd, &f)
	if !f.EnableTelemetry {
		t.Fatal("expected ENABLE_TELEMETRY env to enable telemetry")
	}
}

func TestApplyOtelEnvFlagsRespectsCLI(t *testing.T) {
	cmd := NewRootCommand(logrus.New())
	if err := cmd.ParseFlags([]string{"--enable-telemetry=false"}); err != nil {
		t.Fatal(err)
	}
	f := Flags{EnableTelemetry: false}
	t.Setenv("ENABLE_TELEMETRY", "true")
	applyOtelEnvFlags(cmd, &f)
	if f.EnableTelemetry {
		t.Fatal("CLI flag must win over ENABLE_TELEMETRY env")
	}
}

func TestResolveOTLPEndpointFromEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "10.0.0.1:4317")
	if got := resolveOTLPEndpoint(); got != "10.0.0.1:4317" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if got := resolveOTLPEndpoint(); got == "" {
		t.Fatal("expected default endpoint")
	}
}
