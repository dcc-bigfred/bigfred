package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/version"
)

func TestPrintLayoutsHuman(t *testing.T) {
	var buf bytes.Buffer
	printLayoutsHuman(&buf, []protocol.LayoutResponse{
		{ID: 1, Name: "Klubowa", IsSystem: false, Locked: true},
	})
	got := buf.String()
	if !strings.Contains(got, "Klubowa") || !strings.Contains(got, "yes") || !strings.Contains(got, "no") {
		t.Fatalf("got %q", got)
	}
}

func TestPrintDccBusHuman(t *testing.T) {
	var buf bytes.Buffer
	printDccBusHuman(&buf, []service.DccBusProgramStatus{
		{
			Name: "dcc-bus-2-5", LayoutName: "Klubowa", Status: "running",
			WithrottleEnabled: true, WithrottlePort: 12091,
		},
	})
	got := buf.String()
	if !strings.Contains(got, "12091") || !strings.Contains(got, "-") {
		t.Fatalf("got %q", got)
	}
}

func TestPrintVersionHuman(t *testing.T) {
	var buf bytes.Buffer
	printVersionHuman(&buf, version.Info{Product: "bigfred", Version: "dev"})
	if !strings.Contains(buf.String(), "bigfred") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestInvalidOutputFlag(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"-o", "yaml", "version"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "human or json") {
		t.Fatalf("err=%v", err)
	}
}
