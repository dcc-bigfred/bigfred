package service

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

func TestRadioStopServiceRequiresRedisAndHub(t *testing.T) {
	svc := NewRadioStopService(RadioStopConfig{})
	sess := ws.NewDriveSession(5, "guest", "", 1)
	ok, code := svc.Trigger(context.Background(), sess)
	if ok || code != "dcc_bus_not_configured" {
		t.Fatalf("got ok=%v code=%q", ok, code)
	}
}
