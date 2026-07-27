package service

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func TestScanCommandStationsParsesStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-loco-server")
	payload, _ := json.Marshal([]commandstation.DetectedConnection{
		{Name: "Z21", URI: "udp://192.168.0.111:21105"},
	})
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = dcc-bus ] && [ \"$2\" = scan ]; then\n" +
		"  printf '%s\\n' '" + string(payload) + "'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	// Ensure executable bit survives
	if err := exec.Command("chmod", "+x", script).Run(); err != nil {
		t.Fatal(err)
	}

	got, err := ScanCommandStations(context.Background(), script)
	if err != nil {
		t.Fatalf("ScanCommandStations: %v", err)
	}
	if len(got) != 1 || got[0].URI != "udp://192.168.0.111:21105" {
		t.Fatalf("got %+v", got)
	}
}
