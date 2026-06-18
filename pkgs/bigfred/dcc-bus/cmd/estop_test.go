package cmd

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func testRedis(t *testing.T) (*state.Redis, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rs := state.NewRedis(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 2, 1)
	return rs, mr.Close
}

func TestIsLocoPlacedForward_defaultsForwardWhenNoCache(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()

	r := &Router{redis: rs}
	if !r.isLocoPlacedForward(context.Background(), 3) {
		t.Fatal("expected forward when no cached state")
	}
}

func TestIsLocoPlacedForward_returnsCachedDirection(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()

	if err := rs.StoreLocoCurrentState(ctx, contract.LocoStateWire{
		Address: 3,
		Forward: false,
	}, StateTTL); err != nil {
		t.Fatal(err)
	}

	r := &Router{redis: rs}
	if r.isLocoPlacedForward(ctx, 3) {
		t.Fatal("expected reverse from cache")
	}
}

func TestApplyEmergencyStop_preservesReverseDirection(t *testing.T) {
	t.Parallel()
	type speedCall struct {
		addr    uint16
		speed   uint8
		forward bool
	}
	var calls []speedCall
	st := &commandstation.StubStation{
		SetSpeedFn: func(addr commandstation.LocoAddr, speed uint8, forward bool, _ uint8) error {
			calls = append(calls, speedCall{uint16(addr), speed, forward})
			return nil
		},
	}
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()
	const addr uint16 = 3

	if err := rs.StoreLocoCurrentState(ctx, contract.LocoStateWire{
		Address: addr,
		Speed:   20,
		Forward: false,
	}, StateTTL); err != nil {
		t.Fatal(err)
	}

	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{{Addr: addr}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	r.applyEmergencyStop(ctx, 1, "sess", []uint16{addr}, "test", false)

	if len(calls) != 1 {
		t.Fatalf("expected one SetSpeed, got %d: %v", len(calls), calls)
	}
	if calls[0].addr != addr || calls[0].speed != 1 || calls[0].forward {
		t.Fatalf("SetSpeed = %+v, want addr=3 speed=1 forward=false", calls[0])
	}
	snap, ok, err := rs.GetLocoCurrentState(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || snap.Forward {
		t.Fatalf("stored snap forward=%v ok=%v, want forward=false", snap.Forward, ok)
	}
}

func TestApplyEStopAll_preservesReverseDirection(t *testing.T) {
	t.Parallel()
	var gotForward bool
	st := &commandstation.StubStation{
		SetSpeedFn: func(_ commandstation.LocoAddr, _ uint8, forward bool, _ uint8) error {
			gotForward = forward
			return nil
		},
	}
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()
	const addr uint16 = 7

	if err := rs.StoreLocoCurrentState(ctx, contract.LocoStateWire{
		Address: addr,
		Speed:   10,
		Forward: false,
	}, StateTTL); err != nil {
		t.Fatal(err)
	}

	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{{Addr: addr}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	r.applyEStopAll(ctx, "radio_stop")

	if gotForward {
		t.Fatal("applyEStopAll SetSpeed forward=true, want false")
	}
	snap, ok, err := rs.GetLocoCurrentState(ctx, addr)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || snap.Forward {
		t.Fatalf("stored snap forward=%v ok=%v, want forward=false", snap.Forward, ok)
	}
}
