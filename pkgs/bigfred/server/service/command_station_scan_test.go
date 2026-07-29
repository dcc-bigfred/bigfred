package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func TestStreamScanCommandStationsParsesNDJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-loco-server")
	line1, _ := json.Marshal(commandstation.DetectedConnection{Name: "Z21", URI: "udp://192.168.0.111:21105"})
	line2, _ := json.Marshal(commandstation.DetectedConnection{Name: "Serial", URI: "serial:///dev/ttyUSB0:57600"})
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = dcc-bus ] && [ \"$2\" = scan ]; then\n" +
		"  printf '%s\\n' '" + string(line1) + "'\n" +
		"  printf '%s\\n' '" + string(line2) + "'\n" +
		"  echo boom >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	var got []commandstation.DetectedConnection
	stderr, err := StreamScanCommandStations(context.Background(), script, func(c commandstation.DetectedConnection) error {
		got = append(got, c)
		return nil
	})
	if err == nil {
		t.Fatal("expected exit error")
	}
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].URI != "udp://192.168.0.111:21105" || got[1].URI != "serial:///dev/ttyUSB0:57600" {
		t.Fatalf("uris: %+v", got)
	}
	if !strings.Contains(stderr, "boom") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestBeginCommandStationScanSingleFlight(t *testing.T) {
	release, err := BeginCommandStationScan()
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	_, err = BeginCommandStationScan()
	if !errors.Is(err, ErrScanAlreadyRunning) {
		t.Fatalf("err=%v", err)
	}
}

func TestStreamScanCommandStationsPassesLanPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-loco-server")
	outFile := filepath.Join(dir, "args.txt")
	body := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" > '" + outFile + "'\n" +
		"exit 0\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvLanPrefix, "192.168.1")
	_, err := StreamScanCommandStations(context.Background(), script, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "dcc-bus scan --lan-prefix 192.168.1\n"
	if string(got) != want {
		t.Fatalf("args=%q want %q", got, want)
	}
}

func TestStreamScanCommandStationsCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script helper")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-loco-server")
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = dcc-bus ] && [ \"$2\" = scan ]; then\n" +
		"  exec sleep 30\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := StreamScanCommandStations(ctx, script, nil)
	wg.Wait()
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("cancel took too long: %v", elapsed)
	}
	if err == nil {
		t.Fatal("expected error after cancel")
	}
}

