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

	dccbus "github.com/keskad/loco/pkgs/dcc-bus"
	"github.com/keskad/loco/pkgs/dcc-bus/cliargs"
)

// Flags collects every command-line knob the dcc-bus daemon exposes.
type Flags struct {
	LayoutID         uint
	CommandStationID uint

	BindAddr string
	Port     uint16

	RedisAddr string

	StationName      string
	StationKind      string
	StationURI       string
	StationSpeedSteps uint

	JWTSecret    string
	JWTSecretEnv string

	HeartbeatSecs float64
	DeadmanSecs   float64

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
			cs, err := cliargs.CommandStationFromFlags(
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
	cmd.Flags().StringVar(&f.RedisAddr, "redis-addr", "127.0.0.1:6380", "redis host:port used for state cache and pub/sub")
	cmd.Flags().StringVar(&f.StationName, cliargs.FlagStationName, "", "command station display name (required; set by loco-server)")
	cmd.Flags().StringVar(&f.StationKind, cliargs.FlagStationKind, "", "driver kind: z21 | loconet_serial | loconet_tcp (required)")
	cmd.Flags().StringVar(&f.StationURI, cliargs.FlagStationURI, "", "connection URI for the command station (required)")
	cmd.Flags().UintVar(&f.StationSpeedSteps, cliargs.FlagSpeedSteps, 128, "DCC speed steps (14 or 28 or 128)")
	cmd.Flags().StringVar(&f.JWTSecret, "jwt-secret", "", "JWT signing secret (use --jwt-secret-env to read from an env var instead)")
	cmd.Flags().StringVar(&f.JWTSecretEnv, "jwt-secret-env", "BIGFRED_JWT_SECRET", "env var name to read the JWT secret from when --jwt-secret is empty")
	cmd.Flags().Float64Var(&f.HeartbeatSecs, "heartbeat-secs", 5, "WS keepalive interval the daemon advertises to clients")
	cmd.Flags().Float64Var(&f.DeadmanSecs, "deadman-secs", 12, "idle window after which the daemon applies emergency stop to client subscriptions")
	cmd.Flags().StringSliceVar(&f.AllowedOrigins, "allowed-origin", nil, "explicit WS Origin allow-list (empty == accept any; the reverse proxy on loco-server gates Origin in production)")

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

// Run is a convenience helper for tests / harnesses that want to
// invoke the subcommand without going through cobra plumbing.
func Run(ctx context.Context, log *logrus.Logger, args []string) error {
	c := NewCommand(log)
	c.SetArgs(args)
	return c.ExecuteContext(ctx)
}
