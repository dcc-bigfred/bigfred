package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/netutil"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const scanTimeout = 60 * time.Second

func newScanCommand(log *logrus.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Discover command-station connection URIs on serial ports and the local LAN",
		Long: `One-shot discovery tool. Prints a JSON array of {name,uri} objects to
stdout. Logs go to stderr. Used by loco-server's admin scan API.`,
		RunE: func(c *cobra.Command, _ []string) error {
			return runScan(c.Context(), log)
		},
	}
}

func runScan(parent context.Context, log *logrus.Logger) error {
	if log == nil {
		log = logrus.New()
		log.SetOutput(os.Stderr)
	}
	ctx, cancel := context.WithTimeout(parent, scanTimeout)
	defer cancel()

	scanners := commandstation.MultiAutodetection{
		commandstation.LocoNetSerialAutodetection{},
	}
	prefix, err := netutil.LocalIPv4Prefix()
	if err != nil {
		log.WithError(err).Warn("dcc-bus scan: no local IPv4 prefix; skipping LAN discovery")
	} else {
		log.WithField("subnet", prefix).Info("dcc-bus scan: scanning LAN")
		scanners = append(scanners,
			commandstation.LocoNetTCPAutodetection{SubnetPrefix: prefix},
			commandstation.Z21Autodetection{SubnetPrefix: prefix},
		)
	}

	results, err := scanners.Scan(ctx)
	if err != nil {
		return fmt.Errorf("dcc-bus scan: %w", err)
	}
	if results == nil {
		results = []commandstation.DetectedConnection{}
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(results); err != nil {
		return fmt.Errorf("dcc-bus scan: encode: %w", err)
	}
	return nil
}
