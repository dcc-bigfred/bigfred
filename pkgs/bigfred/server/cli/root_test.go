package cli

import (
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
