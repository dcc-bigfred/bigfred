package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const commandStationScanTimeout = 90 * time.Second

// ScanCommandStations runs `loco-server dcc-bus scan` and parses the JSON
// array of detected connections from stdout.
func ScanCommandStations(ctx context.Context, executable string) ([]commandstation.DetectedConnection, error) {
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("command station scan: resolve executable: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, commandStationScanTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, executable, "dcc-bus", "scan")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command station scan: %w (stderr: %s)", err, truncate(stderr.String(), 512))
	}

	var out []commandstation.DetectedConnection
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("command station scan: parse JSON: %w (stdout: %s)", err, truncate(stdout.String(), 512))
	}
	if out == nil {
		out = []commandstation.DetectedConnection{}
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
