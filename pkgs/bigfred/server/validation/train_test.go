package validation_test

import (
	"errors"
	"strings"
	"testing"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

func TestSanitiseTrainName(t *testing.T) {
	got, err := validation.SanitiseTrainName("  IC 123  ")
	if err != nil || got != "IC 123" {
		t.Fatalf("got %q, %v", got, err)
	}
	_, err = validation.SanitiseTrainName("   ")
	if !errors.Is(err, svcerrors.ErrTrainNameRequired) {
		t.Fatalf("got %v", err)
	}
	long := strings.Repeat("x", validation.MaxTrainNameLen+10)
	got, err = validation.SanitiseTrainName(long)
	if err != nil || len(got) != validation.MaxTrainNameLen {
		t.Fatalf("got len %d, err %v", len(got), err)
	}
}

func TestValidateSpeedMultiplier(t *testing.T) {
	for _, v := range []float64{0.5, 1.0, validation.MaxSpeedMultiplier} {
		if err := validation.ValidateSpeedMultiplier(v); err != nil {
			t.Fatalf("multiplier %v: %v", v, err)
		}
	}
	for _, v := range []float64{0, -1, validation.MaxSpeedMultiplier + 0.1} {
		if err := validation.ValidateSpeedMultiplier(v); !errors.Is(err, svcerrors.ErrTrainMemberMultiplierRange) {
			t.Fatalf("multiplier %v: got %v", v, err)
		}
	}
}

func TestValidateStartDelayMs(t *testing.T) {
	for _, ms := range []int{0, 50, 1000} {
		if err := validation.ValidateStartDelayMs(ms); err != nil {
			t.Fatalf("delay %d: %v", ms, err)
		}
	}
	for _, ms := range []int{-50, 25, 1050} {
		if err := validation.ValidateStartDelayMs(ms); !errors.Is(err, svcerrors.ErrTrainMemberStartDelayRange) {
			t.Fatalf("delay %d: got %v", ms, err)
		}
	}
}

func TestValidateAccelRampMs(t *testing.T) {
	for _, ms := range []int{0, 500, 5000} {
		if err := validation.ValidateAccelRampMs(ms); err != nil {
			t.Fatalf("ramp %d: %v", ms, err)
		}
	}
	for _, ms := range []int{-500, 250, 5500} {
		if err := validation.ValidateAccelRampMs(ms); !errors.Is(err, svcerrors.ErrTrainMemberAccelRampRange) {
			t.Fatalf("ramp %d: got %v", ms, err)
		}
	}
}

func TestValidateAccelRampMaxSteps(t *testing.T) {
	for _, steps := range []int{1, 5, validation.MaxAccelRampMaxSteps} {
		if err := validation.ValidateAccelRampMaxSteps(steps); err != nil {
			t.Fatalf("steps %d: %v", steps, err)
		}
	}
	for _, steps := range []int{0, 11} {
		if err := validation.ValidateAccelRampMaxSteps(steps); !errors.Is(err, svcerrors.ErrTrainMemberAccelRampStepsRange) {
			t.Fatalf("steps %d: got %v", steps, err)
		}
	}
}

func TestValidateBrakeRampMs(t *testing.T) {
	for _, ms := range []int{0, 500, 5000} {
		if err := validation.ValidateBrakeRampMs(ms); err != nil {
			t.Fatalf("ramp %d: %v", ms, err)
		}
	}
	for _, ms := range []int{-500, 250, 5500} {
		if err := validation.ValidateBrakeRampMs(ms); !errors.Is(err, svcerrors.ErrTrainMemberBrakeRampRange) {
			t.Fatalf("ramp %d: got %v", ms, err)
		}
	}
}

func TestValidateBrakeRampMaxSteps(t *testing.T) {
	for _, steps := range []int{1, 5, validation.MaxBrakeRampMaxSteps} {
		if err := validation.ValidateBrakeRampMaxSteps(steps); err != nil {
			t.Fatalf("steps %d: %v", steps, err)
		}
	}
	for _, steps := range []int{0, 11} {
		if err := validation.ValidateBrakeRampMaxSteps(steps); !errors.Is(err, svcerrors.ErrTrainMemberBrakeRampStepsRange) {
			t.Fatalf("steps %d: got %v", steps, err)
		}
	}
}
