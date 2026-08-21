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

func TestUnpairedNonSentinelDriveSendsNotPaired(t *testing.T) {
	srv, clientConn, done := startUnpairedThrottle(t)
	defer closeUnpairedThrottle(t, clientConn, done)

	w, readLine := unpairedIO(t, clientConn)
	w("HUtest-device")
	drainUntil(t, readLine, func(line string) bool { return strings.HasPrefix(line, "HT") })

	w(fmt.Sprintf("M0AS5%sF10", propSep))
	if got := readLine(); got != "HMNot paired" {
		t.Fatalf("unpaired F0 on S5: got %q", got)
	}

	w(fmt.Sprintf("M0AS5%sV30", propSep))
	if got := readLine(); got != "HMNot paired" {
		t.Fatalf("unpaired V on S5: got %q", got)
	}

	w(fmt.Sprintf("M0AS5%sR0", propSep))
	if got := readLine(); got != "HMNot paired" {
		t.Fatalf("unpaired R on S5: got %q", got)
	}

	key := locoKeyForAddr(srv.cfg.PairingAddr)
	w(fmt.Sprintf("M0+%s%s", key, propSep))
	drainUntil(t, readLine, func(line string) bool { return strings.Contains(line, "<;>s1") })

	w(fmt.Sprintf("M0AS5%sF10", propSep))
	if got := readLine(); got != "HMNot paired" {
		t.Fatalf("F0 on S5 while sentinel acquired: got %q", got)
	}

	w(fmt.Sprintf("M0A%s%sF10", key, propSep))
	got := readLine()
	if got == "HMNot paired" {
		t.Fatal("sentinel F0 must not send HMNot paired")
	}
}

func startUnpairedThrottle(t *testing.T) (*Server, net.Conn, chan struct{}) {
	t.Helper()
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
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = serverConn.Close() })
	done := make(chan struct{})
	go func() {
		srv.serveConn(ctx, serverConn)
		close(done)
	}()
	return srv, clientConn, done
}

func closeUnpairedThrottle(t *testing.T, clientConn net.Conn, done chan struct{}) {
	t.Helper()
	_ = clientConn.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveConn did not exit")
	}
}

func unpairedIO(t *testing.T, clientConn net.Conn) (func(string), func() string) {
	t.Helper()
	r := bufio.NewReader(clientConn)
	w := func(line string) {
		t.Helper()
		if err := clientConn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintf(clientConn, "%s\n", line); err != nil {
			t.Fatal(err)
		}
	}
	readLine := func() string {
		t.Helper()
		if err := clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimRight(line, "\r\n")
	}
	return w, readLine
}

func drainUntil(t *testing.T, readLine func() string, match func(string) bool) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if match(readLine()) {
			return
		}
	}
	t.Fatal("drainUntil: terminator not seen")
}
