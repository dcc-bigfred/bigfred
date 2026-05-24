// Command loco-server runs the BigFred HTTP/WebSocket backend.
//
// Build:
//
//	CGO_ENABLED=0 go build -o bin/loco-server ./pkgs/server
//
// Run with defaults (SQLite file in the working directory, API on :8080):
//
//	./bin/loco-server
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/server/cli"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	// Wire a parent context that the cobra command can observe so a
	// double Ctrl-C aborts hard. The cobra Run wires its own
	// secondary signal handler for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cmd := cli.NewRootCommand(log)
	if err := cmd.ExecuteContext(ctx); err != nil {
		log.WithError(err).Error("server exited with error")
		os.Exit(1)
	}
}
