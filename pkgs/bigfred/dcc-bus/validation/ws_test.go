package validation_test

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/validation"
)

func TestLocoSubscribeValid(t *testing.T) {
	v := validation.LocoSubscribe{}
	if !v.Valid(protocol.LocoSubscribePayload{Addresses: []uint16{3, 42}}) {
		t.Fatal("expected non-zero addresses to be valid")
	}
}

func TestLocoSubscribeRejectsEmptyAndZeroAddress(t *testing.T) {
	v := validation.LocoSubscribe{}
	cases := []protocol.LocoSubscribePayload{
		{},
		{Addresses: nil},
		{Addresses: []uint16{0}},
		{Addresses: []uint16{3, 0}},
	}
	for _, p := range cases {
		if v.Valid(p) {
			t.Fatalf("expected invalid payload: %+v", p)
		}
	}
}

func TestSetSpeedValidWithConfiguredSteps(t *testing.T) {
	v := validation.SetSpeed{SpeedSteps: 28}
	if !v.Valid(contract.LocoSetSpeedWire{Address: 3, Speed: 28}) {
		t.Fatal("expected speed at configured steps to be valid")
	}
}

func TestSetSpeedRejectsZeroAddress(t *testing.T) {
	v := validation.SetSpeed{SpeedSteps: 128}
	if v.Valid(contract.LocoSetSpeedWire{Address: 0, Speed: 10}) {
		t.Fatal("expected zero address to be invalid")
	}
}

func TestSetSpeedDefaultStepsCap(t *testing.T) {
	v := validation.SetSpeed{SpeedSteps: 0}
	if !v.Valid(contract.LocoSetSpeedWire{Address: 1, Speed: 128}) {
		t.Fatal("expected speed 128 to be valid when speed steps unset")
	}
	if v.Valid(contract.LocoSetSpeedWire{Address: 1, Speed: 129}) {
		t.Fatal("expected speed above 128 to be invalid when speed steps unset")
	}
}

func TestSetSpeedConfiguredStepsCap(t *testing.T) {
	v := validation.SetSpeed{SpeedSteps: 14}
	if v.Valid(contract.LocoSetSpeedWire{Address: 1, Speed: 15}) {
		t.Fatal("expected speed above configured steps to be invalid")
	}
}

func TestSetFunctionValidAndInvalid(t *testing.T) {
	v := validation.SetFunction{}
	if !v.Valid(contract.LocoSetFunctionWire{Address: 5, Function: 31}) {
		t.Fatal("expected F31 to be valid")
	}
	cases := []contract.LocoSetFunctionWire{
		{Address: 0, Function: 0},
		{Address: 1, Function: 32},
	}
	for _, p := range cases {
		if v.Valid(p) {
			t.Fatalf("expected invalid payload: %+v", p)
		}
	}
}

func TestTrainSetSpeedValidWithConfiguredSteps(t *testing.T) {
	v := validation.TrainSetSpeed{SpeedSteps: 128}
	if !v.Valid(contract.TrainSetSpeedWire{TrainID: 7, Speed: 64}) {
		t.Fatal("expected in-range train speed to be valid")
	}
}

func TestTrainSetSpeedRejectsZeroTrainID(t *testing.T) {
	v := validation.TrainSetSpeed{SpeedSteps: 128}
	if v.Valid(contract.TrainSetSpeedWire{TrainID: 0, Speed: 1}) {
		t.Fatal("expected zero train id to be invalid")
	}
}

func TestTrainSetSpeedDefaultStepsCap(t *testing.T) {
	v := validation.TrainSetSpeed{SpeedSteps: 0}
	if v.Valid(contract.TrainSetSpeedWire{TrainID: 1, Speed: 129}) {
		t.Fatal("expected speed above 128 to be invalid when speed steps unset")
	}
}
