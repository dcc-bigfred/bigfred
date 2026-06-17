package validation_test

import (
	"errors"
	"strings"
	"testing"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

func TestSanitiseInterlockingName(t *testing.T) {
	got, err := validation.SanitiseInterlockingName("  North  ")
	if err != nil || got != "North" {
		t.Fatalf("got %q, %v", got, err)
	}
	_, err = validation.SanitiseInterlockingName("")
	if !errors.Is(err, svcerrors.ErrInterlockingNameRequired) {
		t.Fatalf("empty: got %v", err)
	}
	_, err = validation.SanitiseInterlockingName(strings.Repeat("n", 65))
	if !errors.Is(err, svcerrors.ErrInterlockingNameRequired) {
		t.Fatalf("long: got %v", err)
	}
}

func TestSanitiseInterlockingLocation(t *testing.T) {
	if got := validation.SanitiseInterlockingLocation("  yard A  "); got != "yard A" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("l", 600)
	got := validation.SanitiseInterlockingLocation(long)
	if len(got) != 512 {
		t.Fatalf("len = %d", len(got))
	}
}
