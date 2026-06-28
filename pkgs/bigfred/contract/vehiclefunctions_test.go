package contract

import "testing"

func TestFunctionLabelFallback(t *testing.T) {
	if got := (FunctionDefinition{Num: 3}).FunctionLabel(); got != "F3" {
		t.Fatalf("got %q want F3", got)
	}
	if got := (FunctionDefinition{Num: 0, Icon: "unspecified"}).FunctionLabel(); got != "F0" {
		t.Fatalf("got %q want F0", got)
	}
}

func TestVehicleFunctionsChangedAddrs(t *testing.T) {
	prev := VehicleFunctions{
		Vehicles: []VehicleFunctionCatalogue{
			{Addr: 3, Functions: []FunctionDefinition{{Num: 0, Name: "Light"}}},
			{Addr: 5, Functions: []FunctionDefinition{{Num: 1, Name: "Bell"}}},
		},
	}
	next := VehicleFunctions{
		Vehicles: []VehicleFunctionCatalogue{
			{Addr: 3, Functions: []FunctionDefinition{{Num: 0, Name: "Headlight"}}},
			{Addr: 5, Functions: []FunctionDefinition{{Num: 1, Name: "Bell"}}},
		},
	}
	got := VehicleFunctionsChangedAddrs(prev, next)
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("got %v want [3]", got)
	}
}
