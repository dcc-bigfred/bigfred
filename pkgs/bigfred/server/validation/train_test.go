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
