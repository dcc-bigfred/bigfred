package commandstation

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// DetectedConnection is one discovered command-station connection option
// suitable for BigFred's catalogue (display name + connection URI).
type DetectedConnection struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// Autodetection discovers available command-station connections for a
// single transport family (serial, LocoNet TCP, Z21, …). Callers compose
// multiple implementations when scanning everything.
type Autodetection interface {
	Scan(ctx context.Context) ([]DetectedConnection, error)
}

// MultiAutodetection runs several Autodetection implementations in parallel
// and concatenates their results in slice order. The first scanner error
// cancels the others and is returned.
type MultiAutodetection []Autodetection

func (m MultiAutodetection) Scan(ctx context.Context) ([]DetectedConnection, error) {
	type result struct {
		got []DetectedConnection
		err error
	}
	results := make([]result, len(m))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i, a := range m {
		if a == nil {
			continue
		}
		wg.Add(1)
		go func(i int, a Autodetection) {
			defer wg.Done()
			got, err := a.Scan(ctx)
			results[i] = result{got: got, err: err}
			if err != nil {
				cancel()
			}
		}(i, a)
	}
	wg.Wait()

	out := make([]DetectedConnection, 0)
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		out = append(out, r.got...)
	}
	return out, nil
}

// TCPDialer opens a TCP connection to address (host:port). Overridable in tests.
type TCPDialer func(ctx context.Context, address string) (net.Conn, error)

func defaultTCPDialer(timeout time.Duration) TCPDialer {
	return func(ctx context.Context, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: timeout}
		return d.DialContext(ctx, "tcp", address)
	}
}

// UDPProber sends a UDP datagram to address and waits for any reply.
// Overridable in tests.
type UDPProber func(ctx context.Context, address string, payload []byte) error

func defaultUDPProber(timeout time.Duration) UDPProber {
	return func(ctx context.Context, address string, payload []byte) error {
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "udp", address)
		if err != nil {
			return err
		}
		defer conn.Close()
		deadline, ok := ctx.Deadline()
		if !ok {
			deadline = time.Now().Add(timeout)
		}
		_ = conn.SetDeadline(deadline)
		if _, err := conn.Write(payload); err != nil {
			return err
		}
		buf := make([]byte, 64)
		_, err = conn.Read(buf)
		return err
	}
}

const (
	defaultLANDialTimeout = 200 * time.Millisecond
	defaultLANWorkers     = 32
)

// scanTCPHosts probes hostPrefix.1–255 for openTCP ports in parallel.
// onOpen is called for each successful dial (hostIP, port).
func scanTCPHosts(
	ctx context.Context,
	hostPrefix string,
	ports []int,
	dial TCPDialer,
	onOpen func(host string, port int),
) error {
	if hostPrefix == "" {
		return fmt.Errorf("commandstation: empty host prefix")
	}
	if dial == nil {
		dial = defaultTCPDialer(defaultLANDialTimeout)
	}

	type job struct {
		host string
		port int
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	workers := defaultLANWorkers
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ctx.Err() != nil {
					continue
				}
				addr := fmt.Sprintf("%s:%d", j.host, j.port)
				conn, err := dial(ctx, addr)
				if err != nil {
					continue
				}
				_ = conn.Close()
				onOpen(j.host, j.port)
			}
		}()
	}

	loop:
	for i := 1; i <= 255; i++ {
		host := fmt.Sprintf("%s.%d", hostPrefix, i)
		for _, port := range ports {
			select {
			case <-ctx.Done():
				break loop
			case jobs <- job{host: host, port: port}:
			}
		}
	}
	close(jobs)
	wg.Wait()
	return ctx.Err()
}
