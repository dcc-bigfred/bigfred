package cli

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestStartHTTPMDNSSkippedWhenDisabled(t *testing.T) {
	t.Parallel()
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Must not panic or block when disabled.
	startHTTPMDNS(ctx, log, Flags{MDNS: false, HTTPAddr: "0.0.0.0:8080", MDNSHost: "bigfred"})
}

func TestStartHTTPMDNSSkippedForLoopback(t *testing.T) {
	t.Parallel()
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	startHTTPMDNS(ctx, log, Flags{MDNS: true, HTTPAddr: "127.0.0.1:8080", MDNSHost: "bigfred"})
	<-ctx.Done()
}
