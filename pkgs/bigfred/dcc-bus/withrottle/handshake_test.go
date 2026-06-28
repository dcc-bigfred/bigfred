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
