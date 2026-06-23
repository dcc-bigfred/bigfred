package validation

import (
	"errors"
	"testing"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

func TestSanitiseCommandStationTimingDefaults(t *testing.T) {
	hb, dm, err := SanitiseCommandStationTiming(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hb != 2 || dm != 6 {
		t.Fatalf("got heartbeat=%v deadman=%v", hb, dm)
	}
}

func TestSanitiseCommandStationTimingRejectsDeadmanTooShort(t *testing.T) {
	_, _, err := SanitiseCommandStationTiming(5, 5)
	if !errors.Is(err, svcerrors.ErrCommandStationDeadmanTooShort) {
		t.Fatalf("got %v", err)
	}
}

func TestSanitiseCommandStationSpeedStepsDefaults(t *testing.T) {
	steps, err := SanitiseCommandStationSpeedSteps(0)
	if err != nil {
		t.Fatal(err)
	}
	if steps != 128 {
		t.Fatalf("got %d", steps)
	}
}

func TestSanitiseCommandStationPollIntervalAllowsZero(t *testing.T) {
	ms, err := SanitiseCommandStationPollInterval(0)
	if err != nil || ms != 0 {
		t.Fatalf("got %d, %v", ms, err)
	}
}

func TestSanitiseCommandStationPollIntervalRejectsTooLarge(t *testing.T) {
	_, err := SanitiseCommandStationPollInterval(60001)
	if !errors.Is(err, svcerrors.ErrCommandStationPollIntervalInvalid) {
		t.Fatalf("got %v", err)
	}
}
