package cmd_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/cmd"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/ws"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func newTestRouter(t *testing.T, st commandstation.Station, allowed contract.AllowedVehicles) *cmd.Router {
	t.Helper()
	r, err := cmd.NewRouter(context.Background(), cmd.Config{
		Station:         st,
		Hub:             ws.HubPort(ws.NewHub()),
		LayoutID:        2,
		CommandStationID: 1,
		SpeedSteps:      128,
		Log:             logrus.New(),
		AllowedVehicles: allowed,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
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
	r := newTestRouter(t, st, contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 31}, {Addr: 42}},
	})

	r.ApplyAllowedVehicles(context.Background(), contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 42}},
	})

	if len(speedAddrs) != 1 || speedAddrs[0] != 31 {
		t.Fatalf("expected retire stop for 31, got %v", speedAddrs)
	}
	maxFn := int(service.MaxDCCFunctionNum()) + 1
	if fnOffCount != maxFn {
		t.Fatalf("expected %d fn off, got %d", maxFn, fnOffCount)
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
	_ = newTestRouter(t, st, contract.AllowedVehicles{
		LayoutID: 2,
		Vehicles: []contract.AllowedVehicle{{Addr: 7}},
	})
	if len(speedAddrs) != 0 {
		t.Fatalf("first boot should not retire, got SetSpeed %v", speedAddrs)
	}
}
