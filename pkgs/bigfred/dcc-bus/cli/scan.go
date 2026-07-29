package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/netutil"
	"github.com/keskad/loco/pkgs/bigfred/platform"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const scanTimeout = 60 * time.Second

func newScanCommand(log *logrus.Logger) *cobra.Command {
	var lanPrefix string
	supportsSerial := platform.SupportsLocoNetSerial()
	short := "Discover command-station connection URIs on the local LAN"
	if supportsSerial {
		short = "Discover command-station connection URIs on serial ports and the local LAN"
	}
	long := `One-shot discovery tool. Prints one JSON object {name,uri} per line
(NDJSON) to stdout as connections are found. Logs go to stderr. Used by
loco-server's admin scan WebSocket. Exit code 1 when a scanner fails;
context deadline (scan window) is not treated as failure.

--lan-prefix skips net.InterfaceAddrs (needed on Android where netlink is
denied). Example: --lan-prefix=192.168.0`
	if !supportsSerial {
		long += "\n\nLocoNet serial discovery is not available on this build."
	}
	cmd := &cobra.Command{
		Use:   "scan",
		Short: short,
		Long:  long,
		RunE: func(c *cobra.Command, _ []string) error {
			return runScan(c.Context(), log, lanPrefix)
		},
	}
	cmd.Flags().StringVar(&lanPrefix, "lan-prefix", "",
		"IPv4 /24-style prefix for LAN discovery (e.g. 192.168.0); skips InterfaceAddrs")
	return cmd
}

func runScan(parent context.Context, log *logrus.Logger, lanPrefix string) error {
	if log == nil {
		log = logrus.New()
		log.SetOutput(os.Stderr)
	}
	ctx, cancel := context.WithTimeout(parent, scanTimeout)
	defer cancel()

	prefix, err := resolveLanPrefix(lanPrefix)
	if err != nil {
		// An explicit --lan-prefix that fails validation is an operator error —
		// fail loudly instead of silently running a LAN-less scan that looks
		// like "no stations found". Auto-detect failures stay non-fatal.
		if strings.TrimSpace(lanPrefix) != "" {
			return fmt.Errorf("dcc-bus scan: invalid --lan-prefix: %w", err)
		}
		log.WithError(err).Warn("dcc-bus scan: no local IPv4 prefix; skipping LAN discovery")
		prefix = ""
	} else {
		log.WithField("subnet", prefix).Info("dcc-bus scan: scanning LAN")
	}

	scanners := buildScanAutodetections(platform.SupportsLocoNetSerial(), prefix)

	enc := json.NewEncoder(os.Stdout)
	scanErr := scanners.Scan(ctx, func(c commandstation.DetectedConnection) error {
		if err := enc.Encode(c); err != nil {
			return fmt.Errorf("dcc-bus scan: encode: %w", err)
		}
		_ = os.Stdout.Sync()
		return nil
	})
	if scanErr == nil {
		return nil
	}
	if errors.Is(scanErr, context.DeadlineExceeded) {
		return nil
	}
	log.WithError(scanErr).Error("dcc-bus scan: scanner error")
	return fmt.Errorf("dcc-bus scan: %w", scanErr)
}

// buildScanAutodetections returns the autodetection stack for dcc-bus scan.
// When supportsSerial is false (Android phone build), serial ports are omitted.
// An empty lanPrefix skips LAN scanners.
func buildScanAutodetections(supportsSerial bool, lanPrefix string) commandstation.MultiAutodetection {
	var scanners commandstation.MultiAutodetection
	if supportsSerial {
		scanners = append(scanners, commandstation.LocoNetSerialAutodetection{})
	}
	if lanPrefix != "" {
		scanners = append(scanners,
			commandstation.LocoNetTCPAutodetection{SubnetPrefix: lanPrefix},
			commandstation.Z21Autodetection{SubnetPrefix: lanPrefix},
		)
	}
	return scanners
}

// resolveLanPrefix prefers an explicit --lan-prefix, else auto-detect via netlink.
func resolveLanPrefix(explicit string) (string, error) {
	if p := strings.TrimSpace(explicit); p != "" {
		if err := netutil.ValidateIPv4Prefix(p); err != nil {
			return "", err
		}
		return p, nil
	}
	return netutil.LocalIPv4Prefix()
}
