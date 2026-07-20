// Command bigfred-remote-icmp probes ICMP RTT to active handset IPs listed in
// Redis client snapshots. Configuration is shared with loco-server
// (/data/etc/loco-server.conf).
//
// Build:
//
//	CGO_ENABLED=0 go build -o bin/bigfred-remote-icmp ./pkgs/bigfred/remote-icmp
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	bfotel "github.com/keskad/loco/pkgs/bigfred/otel"
	"github.com/keskad/loco/pkgs/bigfred/server/cli/config"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

type flags struct {
	ConfigPath   string
	RedisAddr    string
	OTLPEndpoint string
	IntervalSecs uint
	ProbeTimeout time.Duration
	LogLevel     string
}

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var f flags
	cmd := &cobra.Command{
		Use:   "bigfred-remote-icmp",
		Short: "ICMP-probe active Z21/WiThrottle handset IPs for Wi-Fi path quality metrics",
		RunE: func(c *cobra.Command, _ []string) error {
			return run(c.Context(), log, f, c.Flags().Changed)
		},
	}
	cmd.Flags().StringVar(&f.ConfigPath, "config", config.DefaultPath, "shared loco-server.conf path")
	cmd.Flags().StringVar(&f.RedisAddr, "redis-addr", "", "redis host:port (overrides conf)")
	cmd.Flags().StringVar(&f.OTLPEndpoint, "otel-endpoint", "", "OTLP/gRPC endpoint (overrides conf; empty uses Alloy default when telemetry enabled)")
	cmd.Flags().UintVar(&f.IntervalSecs, "interval-secs", 0, "probe interval seconds (0 = conf/default 30)")
	cmd.Flags().DurationVar(&f.ProbeTimeout, "probe-timeout", 2*time.Second, "per-target ICMP timeout")
	cmd.Flags().StringVar(&f.LogLevel, "log-level", "", "logrus level (overrides conf)")

	if err := cmd.ExecuteContext(ctx); err != nil {
		log.WithError(err).Error("remote-icmp exited with error")
		os.Exit(1)
	}
}

func run(ctx context.Context, log *logrus.Logger, f flags, changed func(string) bool) error {
	cfgFile := loadSharedConfig(f.ConfigPath, log)
	applyLogLevel(log, f, cfgFile, changed)

	redisAddr := f.RedisAddr
	if !changed("redis-addr") || redisAddr == "" {
		if redisAddr == "" {
			redisAddr = config.RedisDialAddr(cfgFile)
		}
	}
	interval := config.RemoteICMPInterval(cfgFile)
	if changed("interval-secs") && f.IntervalSecs > 0 {
		interval = time.Duration(f.IntervalSecs) * time.Second
	} else if f.IntervalSecs > 0 {
		interval = time.Duration(f.IntervalSecs) * time.Second
	}

	enableTelemetry := cfgFile.EnableTelemetry != nil && *cfgFile.EnableTelemetry
	otelEndpoint := f.OTLPEndpoint
	if otelEndpoint == "" && enableTelemetry {
		otelEndpoint = service.DefaultOTLPEndpoint
	}
	if changed("otel-endpoint") && f.OTLPEndpoint != "" {
		otelEndpoint = f.OTLPEndpoint
	}

	var metrics *Metrics
	if otelEndpoint != "" {
		shutdown, err := bfotel.InitMetrics(ctx, "bigfred-remote-icmp", otelEndpoint)
		if err != nil {
			return fmt.Errorf("otel init: %w", err)
		}
		defer func() { _ = shutdown(context.Background()) }()
		m, err := NewMetrics()
		if err != nil {
			return err
		}
		metrics = m
		log.WithField("endpoint", otelEndpoint).Info("remote-icmp metrics enabled")
	} else {
		log.Info("remote-icmp metrics disabled (telemetry off / no otel-endpoint)")
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.WithError(err).WithField("addr", redisAddr).Warn("redis ping failed; will retry on probe ticks")
	}

	prober, err := NewProber(ProberConfig{
		Redis:    rdb,
		Interval: interval,
		Timeout:  f.ProbeTimeout,
		Metrics:  metrics,
		Log:      log,
	})
	if err != nil {
		return err
	}
	log.WithFields(logrus.Fields{
		"redis":    redisAddr,
		"interval": interval.String(),
		"timeout":  f.ProbeTimeout.String(),
		"config":   f.ConfigPath,
	}).Info("remote-icmp starting")
	return prober.Run(ctx)
}

func loadSharedConfig(path string, log *logrus.Logger) config.File {
	data, err := os.ReadFile(path)
	if err != nil {
		log.WithError(err).WithField("path", path).Warn("shared config missing; using built-in defaults")
		return config.DefaultFile()
	}
	return config.Parse(string(data))
}

func applyLogLevel(log *logrus.Logger, f flags, cfg config.File, changed func(string) bool) {
	level := cfg.LogLevel
	if changed("log-level") && f.LogLevel != "" {
		level = f.LogLevel
	}
	if level == "" {
		return
	}
	parsed, err := logrus.ParseLevel(level)
	if err != nil {
		log.WithError(err).Warn("invalid log level")
		return
	}
	log.SetLevel(parsed)
}
