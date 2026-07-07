package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func TestEnsureBootStop_onFirstNonEmptyRoster(t *testing.T) {
	t.Parallel()

	var speedAddrs []uint16
	st := &commandstation.StubStation{
		SetSpeedFn: func(addr commandstation.LocoAddr, speed uint8, _ bool, _ uint8) error {
			if speed != 1 {
				t.Fatalf("boot stop SetSpeed step = %d, want 1", speed)
			}
			speedAddrs = append(speedAddrs, uint16(addr))
			return nil
		},
	}
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()

	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		BootStopEnabled:  true,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{{Addr: 3}, {Addr: 7}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(speedAddrs) != 2 {
		t.Fatalf("boot stop SetSpeed calls = %d, want 2", len(speedAddrs))
	}

	speedAddrs = nil
	r.ApplyAllowedVehicles(ctx, contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 3}, {Addr: 7}, {Addr: 9}},
	})
	if len(speedAddrs) != 0 {
		t.Fatalf("second roster update SetSpeed calls = %d, want 0", len(speedAddrs))
	}
}

func TestEnsureBootStop_waitsForFirstLayoutList(t *testing.T) {
	t.Parallel()

	var speedAddrs []uint16
	st := &commandstation.StubStation{
		SetSpeedFn: func(addr commandstation.LocoAddr, speed uint8, _ bool, _ uint8) error {
			speedAddrs = append(speedAddrs, uint16(addr))
			return nil
		},
	}
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()

	r, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		BootStopEnabled:  true,
		AllowedVehicles:  contract.AllowedVehicles{LayoutID: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(speedAddrs) != 0 {
		t.Fatalf("empty roster at boot should not stop, got %v", speedAddrs)
	}

	r.ApplyAllowedVehicles(ctx, contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 5}},
	})
	if len(speedAddrs) != 1 || speedAddrs[0] != 5 {
		t.Fatalf("first layout list stop = %v, want [5]", speedAddrs)
	}
}

func TestEnsureBootStop_disabledByDefault(t *testing.T) {
	t.Parallel()

	var speedAddrs []uint16
	st := &commandstation.StubStation{
		SetSpeedFn: func(addr commandstation.LocoAddr, speed uint8, _ bool, _ uint8) error {
			speedAddrs = append(speedAddrs, uint16(addr))
			return nil
		},
	}
	rs, cleanup := testRedis(t)
	defer cleanup()
	ctx := context.Background()

	_, err := NewRouter(ctx, Config{
		Station:          st,
		Hub:              &stubHub{},
		Redis:            rs,
		LayoutID:         2,
		CommandStationID: 1,
		SpeedSteps:       128,
		AllowedVehicles: contract.AllowedVehicles{
			LayoutID: 2,
			Vehicles: []contract.AllowedVehicle{{Addr: 3}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(speedAddrs) != 0 {
		t.Fatalf("boot stop disabled: SetSpeed calls = %d, want 0", len(speedAddrs))
	}
}
