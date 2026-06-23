// Command loco-server-load-test generates sustained throttle traffic against
// a running BigFred stack for performance and soak testing.
//
// Build:
//
//	CGO_ENABLED=0 go build -o bin/loco-server-load-test ./pkgs/bigfred/loadtest
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/loadtest/cli"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cmd := cli.NewCommand(log)
	if err := cmd.ExecuteContext(ctx); err != nil {
		log.WithError(err).Error("load test exited with error")
		os.Exit(1)
	}
}
