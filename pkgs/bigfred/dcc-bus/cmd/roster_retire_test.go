package cmd

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/ws"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func TestRetireRemovedLocoStopsAndClearsFunctions(t *testing.T) {
	t.Parallel()
	var speedAddrs []uint16
	var fnOffCount int
	st := &commandstation.StubStation{
		SetSpeedFn: func(addr commandstation.LocoAddr, _ uint8, _ bool, _ uint8) error {
			speedAddrs = append(speedAddrs, uint16(addr))
			return nil
		},
		SendFnFn: func(_ commandstation.Mode, _ commandstation.LocoAddr, _ commandstation.FuncNum, toggle bool) error {
			if !toggle {
				fnOffCount++
			}
			return nil
		},
	}
	r := &Router{
		station:    st,
		hub:        ws.NewHub(),
		fnCache:    NewFunctionsCache(),
		speedSteps: 128,
	}
	r.fnCache.Set(31, 2, true)
	r.retireRemovedLoco(context.Background(), 31)

	if len(speedAddrs) != 1 || speedAddrs[0] != 31 {
		t.Fatalf("SetSpeed addrs = %v", speedAddrs)
	}
	if fnOffCount != int(maxDCCFunctionNum)+1 {
		t.Fatalf("SendFn off count = %d, want %d", fnOffCount, maxDCCFunctionNum+1)
	}
	if _, ok := r.fnCache.Get(31, 2); ok {
		t.Fatal("fnCache entry should be cleared")
	}
}

func TestApplyAllowedVehiclesRetiresRemovedBeforeNewRoster(t *testing.T) {
	t.Parallel()
	var speedAddrs []uint16
	var fnOffCount int
	st := &commandstation.StubStation{
		SetSpeedFn: func(addr commandstation.LocoAddr, _ uint8, _ bool, _ uint8) error {
			speedAddrs = append(speedAddrs, uint16(addr))
			return nil
		},
		SendFnFn: func(_ commandstation.Mode, _ commandstation.LocoAddr, _ commandstation.FuncNum, toggle bool) error {
			if !toggle {
				fnOffCount++
			}
			return nil
		},
	}
	r := &Router{
		station:    st,
		hub:        ws.NewHub(),
		roster:     security.NewRosterGate(2),
		fnCache:    NewFunctionsCache(),
		layoutID:   2,
		speedSteps: 128,
		log:        logrus.New(),
	}
	r.ApplyAllowedVehicles(context.Background(), contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 31}, {Addr: 42}},
	})
	r.fnCache.Set(31, 1, true)

	r.ApplyAllowedVehicles(context.Background(), contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 42}},
	})

	if len(speedAddrs) != 1 || speedAddrs[0] != 31 {
		t.Fatalf("expected retire stop for 31, got %v", speedAddrs)
	}
	if fnOffCount != int(maxDCCFunctionNum)+1 {
		t.Fatalf("expected %d fn off, got %d", maxDCCFunctionNum+1, fnOffCount)
	}
	if !r.roster.IsLocoAllowedOnLayout(42) {
		t.Fatal("addr 42 should remain on roster")
	}
	if r.roster.IsLocoAllowedOnLayout(31) {
		t.Fatal("addr 31 should be off roster")
	}
}

func TestApplyAllowedVehiclesSkipsRetireOnFirstBoot(t *testing.T) {
	t.Parallel()
	var speedAddrs []uint16
	st := &commandstation.StubStation{
		SetSpeedFn: func(addr commandstation.LocoAddr, _ uint8, _ bool, _ uint8) error {
			speedAddrs = append(speedAddrs, uint16(addr))
			return nil
		},
	}
	r := &Router{
		station:    st,
		roster:     security.NewRosterGate(2),
		layoutID:   2,
		speedSteps: 128,
		log:        logrus.New(),
	}
	r.ApplyAllowedVehicles(context.Background(), contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 7}},
	})
	if len(speedAddrs) != 0 {
		t.Fatalf("first boot should not retire, got SetSpeed %v", speedAddrs)
	}
}
