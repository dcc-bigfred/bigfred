package validation_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

func TestSanitiseCommandStationInputValid(t *testing.T) {
	name, kind, uri, steps, err := validation.SanitiseCommandStationInput(
		"  Z21 Main  ",
		domain.CommandStationKindZ21,
		" udp://192.168.0.1:21105 ",
		0,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Z21 Main" || kind != domain.CommandStationKindZ21 {
		t.Fatalf("got name=%q kind=%q", name, kind)
	}
	if uri != "udp://192.168.0.1:21105" {
		t.Fatalf("uri = %q", uri)
	}
	if steps != 128 {
		t.Fatalf("speed steps = %d, want default 128", steps)
	}
}

func TestSanitiseCommandStationInputRejectsInvalidNameKindSpeed(t *testing.T) {
	cases := []struct {
		name  string
		input string
		kind  domain.CommandStationKind
		steps uint
		want  error
	}{
		{"empty name", "  ", domain.CommandStationKindZ21, 128, svcerrors.ErrCommandStationNameRequired},
		{"long name", strings.Repeat("a", 65), domain.CommandStationKindZ21, 128, svcerrors.ErrCommandStationNameRequired},
		{"invalid kind", "Z21", domain.CommandStationKind(""), 128, svcerrors.ErrCommandStationKindInvalid},
		{"invalid speed", "Z21", domain.CommandStationKindZ21, 99, svcerrors.ErrCommandStationSpeedInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, _, err := validation.SanitiseCommandStationInput(tc.input, tc.kind, "", tc.steps)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSanitiseCommandStationName(t *testing.T) {
	got, err := validation.SanitiseCommandStationName("  Desk  ")
	if err != nil || got != "Desk" {
		t.Fatalf("got %q, %v", got, err)
	}
	_, err = validation.SanitiseCommandStationName("")
	if !errors.Is(err, svcerrors.ErrCommandStationNameRequired) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateCommandStationSpeedSteps(t *testing.T) {
	for _, steps := range []uint{14, 28, 128} {
		if err := validation.ValidateCommandStationSpeedSteps(steps); err != nil {
			t.Fatalf("steps %d: %v", steps, err)
		}
	}
	if err := validation.ValidateCommandStationSpeedSteps(64); !errors.Is(err, svcerrors.ErrCommandStationSpeedInvalid) {
		t.Fatalf("got %v", err)
	}
}
