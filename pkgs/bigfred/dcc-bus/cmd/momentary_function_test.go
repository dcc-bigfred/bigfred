package cmd

import (
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
)

func TestGetMomentaryDef(t *testing.T) {
	t.Parallel()
	r := &Router{
		layoutID:  1,
		functions: service.NewFunctionCatalogueCache(1),
	}
	r.functions.ApplySnapshot(contract.VehicleFunctions{
		LayoutID: 1,
		Vehicles: []contract.VehicleFunctionCatalogue{{
			Addr: 31,
			Functions: []contract.FunctionDefinition{
				{Num: 0, Name: "Light", Momentary: false},
				{Num: 2, Name: "Horn", Momentary: true, DurationMs: 1500},
			},
		}},
	})

	if _, ok := r.getMomentaryDef(31, 0); ok {
		t.Fatal("F0 is not momentary")
	}
	def, ok := r.getMomentaryDef(31, 2)
	if !ok {
		t.Fatal("expected F2 momentary")
	}
	if def.DurationMs != 1500 {
		t.Fatalf("durationMs=%d", def.DurationMs)
	}
	if def.GetMomentaryDuration() != 1500*time.Millisecond {
		t.Fatalf("duration=%v", def.GetMomentaryDuration())
	}
	if _, ok := r.getMomentaryDef(99, 2); ok {
		t.Fatal("unknown addr should miss")
	}
}
