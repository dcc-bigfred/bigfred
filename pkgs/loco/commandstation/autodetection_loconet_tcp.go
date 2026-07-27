package commandstation

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	locoNetTCPBinaryPort = 1234
	locoNetTCPASCIIPort  = 5550
)

// LocoNetTCPAutodetection scans the local /24 subnet for LocoNet-over-TCP
// services (raw binary on 1234, LbServer ASCII on 5550).
type LocoNetTCPAutodetection struct {
	// SubnetPrefix is the first three IPv4 octets, e.g. "192.168.0".
	SubnetPrefix string
	Dial         TCPDialer
	DialTimeout  time.Duration
}

func (a LocoNetTCPAutodetection) Scan(ctx context.Context) ([]DetectedConnection, error) {
	if a.SubnetPrefix == "" {
		return nil, nil
	}
	timeout := a.DialTimeout
	if timeout <= 0 {
		timeout = defaultLANDialTimeout
	}
	dial := a.Dial
	if dial == nil {
		dial = defaultTCPDialer(timeout)
	}

	var (
		mu  sync.Mutex
		out []DetectedConnection
	)
	err := scanTCPHosts(ctx, a.SubnetPrefix, []int{locoNetTCPBinaryPort, locoNetTCPASCIIPort}, dial, func(host string, port int) {
		mu.Lock()
		defer mu.Unlock()
		switch port {
		case locoNetTCPBinaryPort:
			out = append(out, DetectedConnection{
				Name: fmt.Sprintf("LocoNet TCP Binary %s:%d", host, port),
				URI:  fmt.Sprintf("tcp://%s:%d", host, port),
			})
		case locoNetTCPASCIIPort:
			out = append(out, DetectedConnection{
				Name: fmt.Sprintf("LocoNet TCP ASCII %s:%d", host, port),
				URI:  fmt.Sprintf("lbserver://%s:%d", host, port),
			})
		}
	})
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return out, nil
}
