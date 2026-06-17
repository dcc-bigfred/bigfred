package validation_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

func TestSanitiseLogin(t *testing.T) {
	got, err := validation.SanitiseLogin("  driver.1  ")
	if err != nil || got != "driver.1" {
		t.Fatalf("got %q, %v", got, err)
	}
	_, err = validation.SanitiseLogin("")
	if !errors.Is(err, svcerrors.ErrUserLoginRequired) {
		t.Fatalf("empty: got %v", err)
	}
	_, err = validation.SanitiseLogin(strings.Repeat("a", validation.MaxUserLoginLen+1))
	if !errors.Is(err, svcerrors.ErrUserLoginInvalid) {
		t.Fatalf("long: got %v", err)
	}
	_, err = validation.SanitiseLogin("bad login")
	if !errors.Is(err, svcerrors.ErrUserLoginInvalid) {
		t.Fatalf("spaces: got %v", err)
	}
}

func TestValidateUserPIN(t *testing.T) {
	if err := validation.ValidateUserPIN("1234"); err != nil {
		t.Fatalf("valid pin: %v", err)
	}
	cases := []struct {
		pin  string
		want error
	}{
		{"", svcerrors.ErrUserPINRequired},
		{"123", svcerrors.ErrUserPINInvalid},
		{strings.Repeat("1", validation.MaxUserPINLength+1), svcerrors.ErrUserPINInvalid},
		{"12a4", svcerrors.ErrUserPINInvalid},
	}
	for _, tc := range cases {
		if err := validation.ValidateUserPIN(tc.pin); !errors.Is(err, tc.want) {
			t.Fatalf("pin %q: got %v, want %v", tc.pin, err, tc.want)
		}
	}
}

func TestIsPermanentRole(t *testing.T) {
	if !validation.IsPermanentRole(domain.RoleDriver) || !validation.IsPermanentRole(domain.RoleAdmin) {
		t.Fatal("expected driver and admin to be permanent")
	}
	if validation.IsPermanentRole(domain.Role("guest")) {
		t.Fatal("expected guest to be non-permanent")
	}
}
