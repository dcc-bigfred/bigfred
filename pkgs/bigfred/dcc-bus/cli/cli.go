// Package cli wires the `dcc-bus` cobra subcommand on the loco-server
// binary. The command is intentionally headless — every knob travels
// in via CLI flags so supervisord can spawn it directly without
// going through a shared config file.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	dccbus "github.com/keskad/loco/pkgs/bigfred/dcc-bus"
)

// Flags collects every command-line knob the dcc-bus daemon exposes.
type Flags struct {
	LayoutID         uint
	CommandStationID uint

	BindAddr string
	Port     uint16

	RedisAddr string

	StationName       string
	StationKind       string
	StationURI        string
	StationSpeedSteps uint

	JWTSecret    string
	JWTSecretEnv string

	HeartbeatSecs float64
	DeadmanSecs   float64

	PollIntervalMs uint

	EnableTelemetry bool
	OTLPEndpoint    string

	EnableZ21 bool
	Z21Bind   string
	Z21Port   uint16
	Z21IPStickiness bool

	EnableWithrottle bool
	WithrottleBind   string
	WithrottlePort   uint16
	WithrottlePairingAddr uint16
	WithrottleHeartbeatSecs float64

	AllowedOrigins []string
}

// NewCommand returns the cobra subcommand. Mounted onto the
// loco-server root command in pkgs/server/cli/root.go.
func NewCommand(log *logrus.Logger) *cobra.Command {
	if log == nil {
		log = logrus.New()
	}
	var f Flags

	cmd := &cobra.Command{
		Use:   "dcc-bus",
		Short: "Run one dcc-bus daemon for a (layout, command station) pair",
		Long: `dcc-bus is the data-plane sibling of loco-server: it owns the
connection to one DCC command station for one layout, exposes a
WebSocket endpoint for the throttle UI and relays throttle commands
in both directions. Spawned by loco-server through supervisord;
should rarely be invoked manually.`,
		RunE: func(c *cobra.Command, _ []string) error {
			secret, err := resolveSecret(f)
			if err != nil {
				return err
			}
			cs, err := CommandStationFromFlags(
				f.CommandStationID, f.StationName, f.StationKind, f.StationURI, f.StationSpeedSteps,
			)
			if err != nil {
				return fmt.Errorf("dcc-bus station config: %w", err)
			}
			cfg := dccbus.Config{
				LayoutID:         f.LayoutID,
				CommandStationID: f.CommandStationID,
				BindAddr:         f.BindAddr,
				Port:             f.Port,
				CommandStation:   cs,
				RedisAddr:        f.RedisAddr,
				JWTSecret:        secret,
				AllowedOrigins:   f.AllowedOrigins,
				HeartbeatSecs:    f.HeartbeatSecs,
				DeadmanSecs:      f.DeadmanSecs,
				PollIntervalMs:   f.PollIntervalMs,
				EnableTelemetry:  f.EnableTelemetry,
				OTLPEndpoint:     resolveOTLPEndpoint(f),
				EnableZ21:        f.EnableZ21,
				Z21Bind:          f.Z21Bind,
				Z21Port:          f.Z21Port,
				Z21IPStickiness:  f.Z21IPStickiness,
				EnableWithrottle: f.EnableWithrottle,
				WithrottleBind:   f.WithrottleBind,
				WithrottlePort:   f.WithrottlePort,
				WithrottlePairingAddr: f.WithrottlePairingAddr,
				WithrottleHeartbeatSecs: f.WithrottleHeartbeatSecs,
			}
			d, err := dccbus.New(c.Context(), log, cfg)
			if err != nil {
				return fmt.Errorf("dcc-bus init: %w", err)
			}
			defer func() { _ = d.Close() }()
			return d.Run(c.Context())
		},
	}

	cmd.Flags().UintVar(&f.LayoutID, "layout-id", 0, "layout this daemon serves (required)")
	cmd.Flags().UintVar(&f.CommandStationID, "command-station-id", 0, "command station id this daemon talks to (required)")
	cmd.Flags().StringVar(&f.BindAddr, "bind", "127.0.0.1", "interface to bind the WebSocket listener on")
	cmd.Flags().Uint16Var(&f.Port, "port", 0, "TCP port to expose the WebSocket on (required; allocated by loco-server)")
	cmd.Flags().StringVar(&f.RedisAddr, "redis-addr", "127.0.0.1:6379", "redis host:port used for state cache and pub/sub")
	cmd.Flags().StringVar(&f.StationName, FlagStationName, "", "command station display name (required; set by loco-server)")
	cmd.Flags().StringVar(&f.StationKind, FlagStationKind, "", "driver kind: z21 | loconet_serial | loconet_tcp (required)")
	cmd.Flags().StringVar(&f.StationURI, FlagStationURI, "", "connection URI for the command station (required)")
	cmd.Flags().UintVar(&f.StationSpeedSteps, FlagSpeedSteps, 128, "DCC speed steps (14 or 28 or 128)")
	cmd.Flags().StringVar(&f.JWTSecret, "jwt-secret", "", "JWT signing secret (use --jwt-secret-env to read from an env var instead)")
	cmd.Flags().StringVar(&f.JWTSecretEnv, "jwt-secret-env", "BIGFRED_JWT_SECRET", "env var name to read the JWT secret from when --jwt-secret is empty")
	cmd.Flags().Float64Var(&f.HeartbeatSecs, "heartbeat-secs", 2, "WS ping interval the daemon advertises to clients (kept well below --deadman-secs so jitter does not trip the dead-man)")
	cmd.Flags().Float64Var(&f.DeadmanSecs, "deadman-secs", 6, "idle window after which the daemon applies emergency stop to client subscriptions")
	cmd.Flags().UintVar(&f.PollIntervalMs, "poll-interval-ms", 0, "state-feed polling cadence in ms for drivers without push (0 == default)")
	cmd.Flags().BoolVar(&f.EnableTelemetry, "enable-telemetry", false, "record command-station latency histograms (requires --otel-endpoint or OTEL_EXPORTER_OTLP_ENDPOINT)")
	cmd.Flags().StringVar(&f.OTLPEndpoint, "otel-endpoint", "", "OTLP/gRPC metrics endpoint for Alloy (required for --enable-telemetry; defaults to OTEL_EXPORTER_OTLP_ENDPOINT)")
	cmd.Flags().StringSliceVar(&f.AllowedOrigins, "allowed-origin", nil, "explicit WS Origin allow-list (empty == accept any; the reverse proxy on loco-server gates Origin in production)")
	cmd.Flags().BoolVar(&f.EnableZ21, "enable-z21", false, "listen for inbound Z21 handset UDP connections")
	cmd.Flags().BoolVar(&f.Z21IPStickiness, "z21-ip-stickiness", false, "key Z21 handset sessions by client IP only (survives UDP port changes on reconnect)")
	cmd.Flags().StringVar(&f.Z21Bind, "z21-bind", "0.0.0.0", "interface to bind the inbound Z21 UDP listener on")
	cmd.Flags().Uint16Var(&f.Z21Port, "z21-port", 21105, "UDP port for inbound Z21 handset connections")
	cmd.Flags().BoolVar(&f.EnableWithrottle, "enable-withrottle", false, "listen for inbound WiThrottle TCP connections")
	cmd.Flags().StringVar(&f.WithrottleBind, "withrottle-bind", "0.0.0.0", "interface to bind the inbound WiThrottle TCP listener on")
	cmd.Flags().Uint16Var(&f.WithrottlePort, "withrottle-port", 12090, "TCP port for inbound WiThrottle connections")
	cmd.Flags().Uint16Var(&f.WithrottlePairingAddr, "withrottle-pairing-addr", 10239, "DCC address of the WiThrottle pairing sentinel loco")
	cmd.Flags().Float64Var(&f.WithrottleHeartbeatSecs, "withrottle-heartbeat-secs", 10, "WiThrottle dead-man heartbeat window advertised to clients")

	return cmd
}

func resolveSecret(f Flags) ([]byte, error) {
	if f.JWTSecret != "" {
		return []byte(f.JWTSecret), nil
	}
	if env := f.JWTSecretEnv; env != "" {
		if val := os.Getenv(env); val != "" {
			return []byte(val), nil
		}
	}
	return nil, errors.New("dcc-bus: --jwt-secret (or " + f.JWTSecretEnv + " env var) must be set so the daemon can verify session tokens")
}

func resolveOTLPEndpoint(f Flags) string {
	if f.OTLPEndpoint != "" {
		return f.OTLPEndpoint
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
}

// Run is a convenience helper for tests / harnesses that want to
// invoke the subcommand without going through cobra plumbing.
func Run(ctx context.Context, log *logrus.Logger, args []string) error {
	c := NewCommand(log)
	c.SetArgs(args)
	return c.ExecuteContext(ctx)
}
