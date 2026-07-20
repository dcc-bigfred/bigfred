package validation_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

func TestSanitiseVehicleName(t *testing.T) {
	got, err := validation.SanitiseVehicleName("  ET22  ")
	if err != nil || got != "ET22" {
		t.Fatalf("got %q, %v", got, err)
	}
	_, err = validation.SanitiseVehicleName("")
	if !errors.Is(err, svcerrors.ErrVehicleNameRequired) {
		t.Fatalf("got %v", err)
	}
	long := strings.Repeat("v", validation.MaxVehicleNameLen+5)
	got, err = validation.SanitiseVehicleName(long)
	if err != nil || len(got) != validation.MaxVehicleNameLen {
		t.Fatalf("got len %d, err %v", len(got), err)
	}
}

func TestTrimVehicleNumber(t *testing.T) {
	if got := validation.TrimVehicleNumber("  92510  "); got != "92510" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("9", validation.MaxVehicleNumberLen+3)
	got := validation.TrimVehicleNumber(long)
	if len(got) != validation.MaxVehicleNumberLen {
		t.Fatalf("len = %d", len(got))
	}
}

func TestTrimVehicleCarrierTruncatesByRunes(t *testing.T) {
	if got := validation.TrimVehicleCarrier("  PKP Cargo  "); got != "PKP Cargo" {
		t.Fatalf("got %q", got)
	}
	// Multi-byte Polish letters must not be split mid-character.
	long := strings.Repeat("ż", validation.MaxVehicleCarrierLen+5)
	got := validation.TrimVehicleCarrier(long)
	if utf8.RuneCountInString(got) != validation.MaxVehicleCarrierLen {
		t.Fatalf("rune count = %d, want %d", utf8.RuneCountInString(got), validation.MaxVehicleCarrierLen)
	}
	if !utf8.ValidString(got) {
		t.Fatal("truncated carrier is not valid UTF-8")
	}
}

func TestResolveVehicleDeadManFieldsDefaults(t *testing.T) {
	rp1, emerg, opt, err := validation.ResolveVehicleDeadManFields(nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp1 != domain.DefaultVehicleRp1Function ||
		emerg != domain.DefaultVehicleEmergencyLightsFunction ||
		opt != domain.DeadManSwitchStop {
		t.Fatalf("defaults mismatch: rp1=%d emerg=%d opt=%q", rp1, emerg, opt)
	}
}

func TestResolveVehicleDeadManFieldsCustomValid(t *testing.T) {
	rp1Fn := uint8(5)
	emergFn := uint8(1)
	opt := domain.DeadManSwitchStopHorn
	rp1, emerg, gotOpt, err := validation.ResolveVehicleDeadManFields(&rp1Fn, &emergFn, &opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rp1 != 5 || emerg != 1 || gotOpt != domain.DeadManSwitchStopHorn {
		t.Fatalf("got rp1=%d emerg=%d opt=%q", rp1, emerg, gotOpt)
	}
}

func TestResolveVehicleDeadManFieldsRejectsInvalid(t *testing.T) {
	badFn := uint8(32)
	_, _, _, err := validation.ResolveVehicleDeadManFields(&badFn, nil, nil)
	if !errors.Is(err, svcerrors.ErrVehicleDccFunctionInvalid) {
		t.Fatalf("rp1: got %v", err)
	}
	badOpt := domain.DeadManSwitchOption("invalid")
	_, _, _, err = validation.ResolveVehicleDeadManFields(nil, nil, &badOpt)
	if !errors.Is(err, svcerrors.ErrVehicleDeadManSwitchInvalid) {
		t.Fatalf("dms: got %v", err)
	}
}
