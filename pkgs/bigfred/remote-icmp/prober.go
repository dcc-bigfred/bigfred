package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ProberConfig tunes the ICMP probe loop.
type ProberConfig struct {
	Redis    *redis.Client
	Interval time.Duration
	Timeout  time.Duration
	Metrics  *Metrics
	Log      *logrus.Logger
}

// Prober periodically ICMP-pings handset IPs from Redis snapshots.
type Prober struct {
	cfg   ProberConfig
	conn  *icmp.PacketConn
	ident int
	seq   uint16
}

// NewProber opens an unprivileged ICMP socket (udp4 / IPPROTO_ICMP).
func NewProber(cfg ProberConfig) (*Prober, error) {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.Log == nil {
		cfg.Log = logrus.New()
	}
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("icmp listen udp4: %w (need CAP_NET_RAW or net.ipv4.ping_group_range)", err)
	}
	return &Prober{
		cfg:   cfg,
		conn:  conn,
		ident: os.Getpid() & 0xffff,
	}, nil
}

// Run probes until ctx is cancelled.
func (p *Prober) Run(ctx context.Context) error {
	defer func() { _ = p.conn.Close() }()

	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	p.round(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			p.round(ctx)
		}
	}
}

func (p *Prober) round(ctx context.Context) {
	// Targets are probed sequentially; many timeouts can push a round past the
	// configured interval. Fine for a handful of handsets; add bounded concurrency
	// if client counts grow.
	targets, err := LoadProbeTargets(ctx, p.cfg.Redis)
	if err != nil {
		p.cfg.Log.WithError(err).Warn("load probe targets")
		return
	}
	if len(targets) == 0 {
		p.cfg.Log.Debug("no handset IPs to probe")
		return
	}
	for _, t := range targets {
		if ctx.Err() != nil {
			return
		}
		p.cfg.Metrics.RecordProbe(t)
		rtt, err := p.ping(ctx, t.IP)
		if err != nil {
			p.cfg.Log.WithError(err).WithFields(logrus.Fields{
				"ip":       t.IP.String(),
				"protocol": t.Protocol,
				"login":    t.Login,
			}).Debug("icmp probe failed")
			p.cfg.Metrics.RecordTimeout(t)
			continue
		}
		p.cfg.Metrics.RecordRTT(t, rtt)
		p.cfg.Log.WithFields(logrus.Fields{
			"ip":       t.IP.String(),
			"protocol": t.Protocol,
			"login":    t.Login,
			"rtt_ms":   float64(rtt.Microseconds()) / 1000,
		}).Debug("icmp probe ok")
	}
}

func (p *Prober) ping(ctx context.Context, ip net.IP) (time.Duration, error) {
	p.seq++
	seq := p.seq
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   p.ident,
			Seq:  int(seq),
			Data: []byte("bigfred-remote-icmp"),
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}

	dst := &net.UDPAddr{IP: ip}
	deadline := time.Now().Add(p.cfg.Timeout)
	if err := p.conn.SetDeadline(deadline); err != nil {
		return 0, err
	}

	start := time.Now()
	if _, err := p.conn.WriteTo(wb, dst); err != nil {
		return 0, fmt.Errorf("write: %w", err)
	}

	rb := make([]byte, 1500)
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		n, peer, err := p.conn.ReadFrom(rb)
		if err != nil {
			return 0, fmt.Errorf("read: %w", err)
		}
		peerIP := peerAddrIP(peer)
		if peerIP == nil || !peerIP.Equal(ip) {
			continue
		}
		rm, err := icmp.ParseMessage(1, rb[:n])
		if err != nil {
			continue
		}
		echo, ok := rm.Body.(*icmp.Echo)
		if !ok || rm.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		if echo.ID != p.ident || echo.Seq != int(seq) {
			continue
		}
		return time.Since(start), nil
	}
}

func peerAddrIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP
	case *net.IPAddr:
		return a.IP
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return net.ParseIP(addr.String())
		}
		return net.ParseIP(host)
	}
}
