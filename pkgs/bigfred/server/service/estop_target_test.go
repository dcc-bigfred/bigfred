package service

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

func TestEStopTargetServiceRequiresDccBus(t *testing.T) {
	svc := NewEStopTargetService(EStopTargetConfig{})
	sess := ws.NewDriveSession(5, "guest", 1)
	ok, code := svc.Trigger(context.Background(), sess, "vehicle", "V-1")
	if ok || code != "dcc_bus_not_configured" {
		t.Fatalf("got ok=%v code=%q", ok, code)
	}
}
