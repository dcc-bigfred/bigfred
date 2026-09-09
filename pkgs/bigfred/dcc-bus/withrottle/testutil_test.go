package withrottle

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/contract"
)

func startWT(t *testing.T, cfg Config) *Server {
	t.Helper()
	cfg.Bind = "127.0.0.1"
	cfg.Port = 0
	started := make(chan struct{})
	prev := cfg.OnListening
	cfg.OnListening = func(ctx context.Context) {
		close(started)
		if prev != nil {
			prev(ctx)
		}
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()
	select {
	case <-started:
	case err := <-errCh:
		cancel()
		t.Fatalf("listen: %v", err)
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("listen timeout")
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
	})
	return srv
}

func dialWT(t *testing.T, srv *Server) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	return conn, bufio.NewReader(conn)
}

func wtWrite(t *testing.T, conn net.Conn, line string) {
	t.Helper()
	if _, err := fmt.Fprintln(conn, line); err != nil {
		t.Fatal(err)
	}
}

func wtRead(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line
}

func drainUntilLine(t *testing.T, r *bufio.Reader, match func(string) bool) string {
	t.Helper()
	for i := 0; i < 400; i++ {
		line := wtRead(t, r)
		if match(line) {
			return line
		}
	}
	t.Fatal("drainUntil: terminator not seen")
	return ""
}

// drainQuiet discards every line that arrives within d (including lines still
// in the kernel socket buffer, which bufio.Reader.Buffered does not see) and
// then restores the long test deadline.
func drainQuiet(t *testing.T, conn net.Conn, r *bufio.Reader, d time.Duration) int {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(d))
	n := 0
	for {
		if _, err := r.ReadString('\n'); err != nil {
			break
		}
		n++
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return n
}

func handshakeHU(t *testing.T, conn net.Conn, r *bufio.Reader, device string) {
	t.Helper()
	wtWrite(t, conn, "HU"+device)
	drainUntilLine(t, r, func(line string) bool { return len(line) >= 2 && line[:2] == "HT" })
}

func pairDevice(srv *Server, device string) {
	key := ClientKeyForDevice(device)
	srv.registry.SetPaired(key, &contract.RemoteSessionWire{
		ClientKey:        key,
		UserID:           7,
		AllowAllVehicles: true,
	})
}
