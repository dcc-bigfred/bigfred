// Package cli wires the loco-server-load-test cobra command.
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/keskad/loco/pkgs/bigfred/loadtest/auth"
	"github.com/keskad/loco/pkgs/bigfred/loadtest/dccbus"
	"github.com/keskad/loco/pkgs/bigfred/loadtest/httpapi"
	"github.com/keskad/loco/pkgs/bigfred/loadtest/sim"
)

// Flags collects every command-line knob exposed by loco-server-load-test.
type Flags struct {
	HTTPAddr  string
	DccBusWS  string
	UserLogin string
	PIN       string
	LayoutID  uint
	MaxSpeed       uint
	ShuttleLegSecs float64
	HornFunc       string
	HornMinSecs    float64
	HornMaxSecs    float64
	HornPulseSecs  float64
	WithoutF1      bool
	Vehicles       []string
}

// NewCommand returns the top-level cobra command.
func NewCommand(log *logrus.Logger) *cobra.Command {
	if log == nil {
		log = logrus.New()
	}
	var f Flags

	cmd := &cobra.Command{
		Use:   "loco-server-load-test",
		Short: "Generate sustained throttle traffic for BigFred performance testing",
		Long: `loco-server-load-test authenticates against loco-server, opens a
dcc-bus WebSocket session and continuously drives the listed vehicles
(or every owned on-layout vehicle when --vehicles is omitted) with a
simple speed/function pattern until interrupted.`,
		RunE: func(c *cobra.Command, _ []string) error {
			return run(c.Context(), log, f)
		},
	}

	cmd.Flags().StringVar(&f.HTTPAddr, "http-addr", "http://localhost:8080", "loco-server HTTP base URL")
	cmd.Flags().StringVar(&f.DccBusWS, "dcc-bus-ws", "ws://127.0.0.1:9200/ws", "dcc-bus WebSocket URL (ws:// or wss://)")
	cmd.Flags().StringVar(&f.UserLogin, "user", "admin", "user login")
	cmd.Flags().StringVar(&f.PIN, "pin", "123456", "user PIN")
	cmd.Flags().UintVar(&f.LayoutID, "layout-id", 0, "layout id to pin the session to (required)")
	cmd.Flags().UintVar(&f.MaxSpeed, "max-speed", 3, "maximum DCC speed step to apply when driving (0–127)")
	cmd.Flags().Float64Var(&f.ShuttleLegSecs, "shuttle-leg-secs", 5, "seconds to drive each shuttle leg before reversing direction")
	cmd.Flags().StringVar(&f.HornFunc, "horn-func", "F2", "horn function to pulse periodically (e.g. F2, F3)")
	cmd.Flags().Float64Var(&f.HornMinSecs, "horn-min-secs", 5, "minimum seconds between horn pulses for each vehicle")
	cmd.Flags().Float64Var(&f.HornMaxSecs, "horn-max-secs", 15, "maximum seconds between horn pulses for each vehicle")
	cmd.Flags().Float64Var(&f.HornPulseSecs, "horn-pulse-secs", 1, "seconds to keep the horn function enabled per pulse")
	cmd.Flags().BoolVar(&f.WithoutF1, "without-f1", false, "do not enable F1 at startup")
	cmd.Flags().StringSliceVar(&f.Vehicles, "vehicles", nil, "vehicle ids to drive (default: all owned, on-layout vehicles with a DCC address)")

	_ = cmd.MarkFlagRequired("layout-id")

	return cmd
}

func run(ctx context.Context, log *logrus.Logger, f Flags) error {
	vehicleIDs := normalizeVehicleIDs(f.Vehicles)
	if f.MaxSpeed == 0 {
		return fmt.Errorf("max-speed must be greater than 0")
	}
	if f.MaxSpeed > 127 {
		return fmt.Errorf("max-speed must be at most 127")
	}
	if f.ShuttleLegSecs <= 0 {
		return fmt.Errorf("shuttle-leg-secs must be greater than 0")
	}
	hornFunc, err := parseDCCFunction(f.HornFunc)
	if err != nil {
		return fmt.Errorf("horn-func: %w", err)
	}
	if f.HornMinSecs <= 0 {
		return fmt.Errorf("horn-min-secs must be greater than 0")
	}
	if f.HornMaxSecs <= 0 {
		return fmt.Errorf("horn-max-secs must be greater than 0")
	}
	if f.HornMaxSecs < f.HornMinSecs {
		return fmt.Errorf("horn-max-secs must be >= horn-min-secs")
	}
	if f.HornPulseSecs <= 0 {
		return fmt.Errorf("horn-pulse-secs must be greater than 0")
	}

	log.WithFields(logrus.Fields{
		"http":           f.HTTPAddr,
		"dccBus":         f.DccBusWS,
		"user":           f.UserLogin,
		"layoutId":       f.LayoutID,
		"maxSpeed":       f.MaxSpeed,
		"shuttleLegSecs": f.ShuttleLegSecs,
		"hornFunc":       f.HornFunc,
		"hornMinSecs":    f.HornMinSecs,
		"hornMaxSecs":    f.HornMaxSecs,
		"hornPulseSecs":  f.HornPulseSecs,
		"withoutF1":      f.WithoutF1,
		"vehicles":       vehicleIDsOrAll(vehicleIDs),
	}).Info("starting load test")

	session, err := auth.Login(ctx, f.HTTPAddr, f.UserLogin, f.PIN, f.LayoutID)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	log.WithField("userId", session.UserID).Info("authenticated")

	api := httpapi.NewClient(f.HTTPAddr, session.HTTP)
	locos, err := api.DiscoverDriveableVehicles(ctx, session.UserID, vehicleIDs)
	if err != nil {
		return fmt.Errorf("resolve vehicles: %w", err)
	}
	if len(vehicleIDs) == 0 {
		log.WithField("count", len(locos)).Info("discovered driveable vehicles")
	}
	for _, l := range locos {
		log.WithFields(logrus.Fields{
			"vehicleId": l.VehicleID,
			"address":   l.Address,
		}).Info("resolved vehicle")
	}

	bus, err := dccbus.Connect(ctx, f.DccBusWS, session.Token, log)
	if err != nil {
		return fmt.Errorf("dcc-bus connect: %w", err)
	}
	defer bus.Close()

	addrs := make([]uint16, len(locos))
	for i, l := range locos {
		addrs[i] = l.Address
	}
	if err := bus.Subscribe(ctx, addrs); err != nil {
		return fmt.Errorf("loco.subscribe: %w", err)
	}
	// Slot acquire on subscribe is async in dcc-bus; give LocoNet a moment
	// before the per-vehicle drive loops start hammering SetSpeed.
	time.Sleep(500 * time.Millisecond)

	driver := sim.New(bus, log, sim.Config{
		MaxSpeed:          uint8(f.MaxSpeed),
		LegDuration:       time.Duration(f.ShuttleLegSecs * float64(time.Second)),
		WithoutF1:         f.WithoutF1,
		HornFunction:      hornFunc,
		HornMinInterval:   time.Duration(f.HornMinSecs * float64(time.Second)),
		HornMaxInterval:   time.Duration(f.HornMaxSecs * float64(time.Second)),
		HornPulseDuration: time.Duration(f.HornPulseSecs * float64(time.Second)),
	})
	return driver.Run(ctx, locos)
}

func normalizeVehicleIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, raw := range ids {
		for _, part := range strings.Split(raw, ",") {
			id := strings.TrimSpace(part)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func vehicleIDsOrAll(ids []string) any {
	if len(ids) == 0 {
		return "all"
	}
	return ids
}
