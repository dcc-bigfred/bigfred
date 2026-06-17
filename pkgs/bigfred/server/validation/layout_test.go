package validation_test

import (
	"errors"
	"testing"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/validation"
)

func TestValidateLayoutAdminPIN(t *testing.T) {
	for _, pin := range []string{"1234", "12345678"} {
		if err := validation.ValidateLayoutAdminPIN(pin); err != nil {
			t.Fatalf("pin %q: %v", pin, err)
		}
	}
	cases := []struct {
		pin  string
		want error
	}{
		{"123", svcerrors.ErrLayoutAdminPINInvalid},
		{"123456789", svcerrors.ErrLayoutAdminPINInvalid},
		{"12a4", svcerrors.ErrLayoutAdminPINInvalid},
	}
	for _, tc := range cases {
		if err := validation.ValidateLayoutAdminPIN(tc.pin); !errors.Is(err, tc.want) {
			t.Fatalf("pin %q: got %v", tc.pin, err)
		}
	}
}
