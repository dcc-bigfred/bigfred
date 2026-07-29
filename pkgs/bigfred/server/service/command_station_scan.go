package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

// EnvLanPrefix is set by the Android host process so StreamScanCommandStations
// can pass --lan-prefix (net.InterfaceAddrs / netlink is denied for apps).
const EnvLanPrefix = "BIGFRED_LAN_PREFIX"

const commandStationScanTimeout = 65 * time.Second

const scanErrorDetailLimit = 4096

// ErrScanAlreadyRunning is returned when a second scan is requested while
// another is still in progress (single-flight).
var ErrScanAlreadyRunning = errors.New("scan already running")

var (
	scanGateMu sync.Mutex
	scanBusy   bool
)

// BeginCommandStationScan acquires the single-flight lock. Caller must
// invoke the returned release function (typically via defer).
func BeginCommandStationScan() (release func(), err error) {
	scanGateMu.Lock()
	defer scanGateMu.Unlock()
	if scanBusy {
		return nil, ErrScanAlreadyRunning
	}
	scanBusy = true
	return func() {
		scanGateMu.Lock()
		scanBusy = false
		scanGateMu.Unlock()
	}, nil
}

// StreamScanCommandStations runs `loco-server dcc-bus scan`, calls onHit for
// each NDJSON line on stdout, and returns truncated stderr plus the process
// wait error (nil on exit 0).
func StreamScanCommandStations(
	ctx context.Context,
	executable string,
	onHit func(commandstation.DetectedConnection) error,
) (stderr string, err error) {
	if executable == "" {
		var resolveErr error
		executable, resolveErr = os.Executable()
		if resolveErr != nil {
			return "", fmt.Errorf("command station scan: resolve executable: %w", resolveErr)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, commandStationScanTimeout)
	defer cancel()

	cmdArgs := []string{"dcc-bus", "scan"}
	if prefix := strings.TrimSpace(os.Getenv(EnvLanPrefix)); prefix != "" {
		cmdArgs = append(cmdArgs, "--lan-prefix", prefix)
	}
	cmd := exec.CommandContext(ctx, executable, cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("command station scan: stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("command station scan: start: %w", err)
	}

	sc := bufio.NewScanner(stdout)
	// LAN scans can emit many lines; keep a modest ceiling.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var hit commandstation.DetectedConnection
		if err := json.Unmarshal(line, &hit); err != nil {
			continue
		}
		if hit.URI == "" {
			continue
		}
		if onHit != nil {
			if err := onHit(hit); err != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return truncate(stderrBuf.String(), scanErrorDetailLimit), err
			}
		}
	}
	if err := sc.Err(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return truncate(stderrBuf.String(), scanErrorDetailLimit), fmt.Errorf("command station scan: read stdout: %w", err)
	}

	waitErr := cmd.Wait()
	return truncate(stderrBuf.String(), scanErrorDetailLimit), waitErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
