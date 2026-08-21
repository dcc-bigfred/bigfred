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

func TestHUTriggersInitialBurst(t *testing.T) {
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

	if _, err := fmt.Fprintf(clientConn, "HUengine-driver\n"); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(clientConn)
	first, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(first, "\r\n"); got != "VN2.0" {
		t.Fatalf("first line after HU: got %q want VN2.0", got)
	}

	second, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimRight(second, "\r\n"); got != "*10" {
		t.Fatalf("second line after HU: got %q want *10", got)
	}

	_ = clientConn.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveConn did not exit after client close")
	}
}

func TestHUReconnectWithSameDeviceGetsFreshBurst(t *testing.T) {
	srv, err := New(Config{
		LayoutID:         1,
		CommandStationID: 1,
		HeartbeatSecs:    10,
		TrackPowerOn:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstClient, firstServer := net.Pipe()
	firstDone := make(chan struct{})
	go func() {
		srv.serveConn(ctx, firstServer)
		close(firstDone)
	}()
	if _, err := fmt.Fprintln(firstClient, "HUsame-device"); err != nil {
		t.Fatal(err)
	}
	assertInitialBurst(t, firstClient)

	secondClient, secondServer := net.Pipe()
	secondDone := make(chan struct{})
	go func() {
		srv.serveConn(ctx, secondServer)
		close(secondDone)
	}()
	if _, err := fmt.Fprintln(secondClient, "HUsame-device"); err != nil {
		t.Fatal(err)
	}
	assertInitialBurst(t, secondClient)

	_ = secondClient.Close()
	_ = firstClient.Close()
	for name, done := range map[string]<-chan struct{}{
		"first serveConn":  firstDone,
		"second serveConn": secondDone,
	} {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s did not exit", name)
		}
	}
}

func assertInitialBurst(t *testing.T, conn net.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(conn)
	sawRoster := false
	for i := 0; i < 5; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch i {
		case 0:
			if line != "VN2.0" {
				t.Fatalf("first burst line: got %q want VN2.0", line)
			}
		case 1:
			if line != "*10" {
				t.Fatalf("second burst line: got %q want *10", line)
			}
		}
		sawRoster = sawRoster || strings.HasPrefix(line, "RL")
	}
	if !sawRoster {
		t.Fatal("initial burst did not contain roster")
	}
}
