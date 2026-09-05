package validation

import (
	"errors"
	"testing"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/domain"
	svcerrors "github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/errors"
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

func TestValidateWithrottlePortConflict(t *testing.T) {
	kind := domain.CommandStationKindWiThrottle
	if err := ValidateWithrottlePortConflict(kind, "withrottle://jmri.local:12090", true, 12090); err != nil {
		t.Fatalf("remote host must be allowed: %v", err)
	}
	if err := ValidateWithrottlePortConflict(kind, "withrottle://127.0.0.1:12090", true, 12090); !errors.Is(err, svcerrors.ErrCommandStationWithrottlePortConflict) {
		t.Fatalf("loopback same port: got %v", err)
	}
	if err := ValidateWithrottlePortConflict(kind, "withrottle://127.0.0.1:12090", true, 12091); err != nil {
		t.Fatalf("different inbound port must be allowed: %v", err)
	}
	if err := ValidateWithrottlePortConflict(kind, "withrottle://127.0.0.1:12090", false, 12090); err != nil {
		t.Fatalf("inbound disabled must be allowed: %v", err)
	}
	if err := ValidateWithrottlePortConflict(domain.CommandStationKindZ21, "udp://127.0.0.1:21105", true, 12090); err != nil {
		t.Fatalf("non-withrottle kind must be ignored: %v", err)
	}
}
