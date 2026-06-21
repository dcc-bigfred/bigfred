package contract_test

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func TestEffectiveMemberSpeed(t *testing.T) {
	tests := []struct {
		leading    uint8
		multiplier float64
		max        uint8
		want       uint8
	}{
		{0, 1.5, 28, 0},
		{10, 1.0, 28, 10},
		{10, 1.2, 28, 12},
		{25, 1.3, 28, 28},
		{10, 0, 28, 10},
	}
	for _, tc := range tests {
		got := contract.EffectiveMemberSpeed(tc.leading, tc.multiplier, tc.max)
		if got != tc.want {
			t.Fatalf("EffectiveMemberSpeed(%d, %v, %d) = %d, want %d",
				tc.leading, tc.multiplier, tc.max, got, tc.want)
		}
	}
}

func TestDefinedTrainLeadingMember(t *testing.T) {
	addr1 := uint16(1)
	addr3 := uint16(3)
	train := contract.DefinedTrain{
		Members: []contract.DefinedTrainMember{
			{VehicleID: "V-10", Position: 0, Addr: nil},
			{VehicleID: "V-11", Position: 1, Addr: &addr1},
			{VehicleID: "V-12", Position: 2, Addr: &addr3},
		},
	}
	leading, ok := train.LeadingMember()
	if !ok || leading.VehicleID != "V-11" || leading.Addr == nil || *leading.Addr != 1 {
		t.Fatalf("LeadingMember() = %+v, %v", leading, ok)
	}
}

func TestDefinedTrainLeadingMemberSkipsExcluded(t *testing.T) {
	addr1 := uint16(1)
	addr3 := uint16(3)
	train := contract.DefinedTrain{
		Members: []contract.DefinedTrainMember{
			{VehicleID: "V-11", Position: 0, Addr: &addr1, ExcludeFromSpeed: true},
			{VehicleID: "V-12", Position: 1, Addr: &addr3},
		},
	}
	leading, ok := train.LeadingMember()
	if !ok || leading.VehicleID != "V-12" {
		t.Fatalf("LeadingMember() = %+v, %v", leading, ok)
	}
}

func TestDefinedTrainCanDrive(t *testing.T) {
	train := contract.DefinedTrain{ControllerUserIDs: []uint{1, 5}}
	if !train.CanDrive(5) {
		t.Fatal("expected user 5 to drive")
	}
	if train.CanDrive(9) {
		t.Fatal("expected user 9 denied")
	}
}

func TestMaxSpeedForSpeedSteps(t *testing.T) {
	if got := contract.MaxSpeedForSpeedSteps(14); got != 15 {
		t.Fatalf("14-step max = %d", got)
	}
	if got := contract.MaxSpeedForSpeedSteps(28); got != 28 {
		t.Fatalf("28-step max = %d", got)
	}
	if got := contract.MaxSpeedForSpeedSteps(128); got != 127 {
		t.Fatalf("128-step max = %d", got)
	}
}
