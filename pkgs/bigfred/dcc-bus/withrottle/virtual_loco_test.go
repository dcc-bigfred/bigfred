package withrottle

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestUnpairedSentinelSpeedVirtualEcho(t *testing.T) {
	srv, err := New(Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		srv.serveConn(ctx, serverConn)
		close(done)
	}()

	w := func(line string) {
		t.Helper()
		if _, err := fmt.Fprintf(clientConn, "%s\n", line); err != nil {
			t.Fatal(err)
		}
	}
	readLine := func() string {
		t.Helper()
		r := bufio.NewReader(clientConn)
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimRight(line, "\r\n")
	}

	w("HUtest-device")
	for i := 0; i < 5; i++ {
		_ = readLine()
	}

	key := locoKeyForAddr(srv.cfg.PairingAddr)
	w(fmt.Sprintf("M0+%s%s", key, propSep))
	for i := 0; i < 14; i++ {
		_ = readLine()
	}

	w(fmt.Sprintf("M0A%s%sV30", key, propSep))
	var gotV bool
	for i := 0; i < 40; i++ {
		line := readLine()
		if strings.Contains(line, "<;>V30") {
			gotV = true
			break
		}
	}
	if !gotV {
		t.Fatal("expected M…A…V30 echo for unpaired sentinel throttle")
	}
	if !srv.virtual.HasClient("withrottle:test-device") {
		t.Fatal("expected virtual loco state for WiThrottle client")
	}

	_ = clientConn.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveConn did not exit")
	}
}

func TestClearVirtualLocoRemovesStore(t *testing.T) {
	srv, err := New(Config{LayoutID: 1, CommandStationID: 1})
	if err != nil {
		t.Fatal(err)
	}
	key := "withrottle:device"
	srv.virtual.SetSpeed(key, srv.cfg.PairingAddr, 10, true)
	srv.clearVirtualLoco(key)
	if srv.virtual.HasClient(key) {
		t.Fatal("clearVirtualLoco should remove client state")
	}
}
