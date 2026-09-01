package validation_test

import (
	"errors"
	"strings"
	"testing"

	svcerrors "github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/errors"
	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/validation"
)

func TestSanitiseVehicleTemplateName(t *testing.T) {
	got, err := validation.SanitiseVehicleTemplateName("  EMU set  ")
	if err != nil || got != "EMU set" {
		t.Fatalf("got %q, %v", got, err)
	}
	_, err = validation.SanitiseVehicleTemplateName(" ")
	if !errors.Is(err, svcerrors.ErrVehicleTemplateNameRequired) {
		t.Fatalf("got %v", err)
	}
	long := strings.Repeat("t", validation.MaxVehicleTemplateNameLen+4)
	got, err = validation.SanitiseVehicleTemplateName(long)
	if err != nil || len(got) != validation.MaxVehicleTemplateNameLen {
		t.Fatalf("got len %d, err %v", len(got), err)
	}
}
