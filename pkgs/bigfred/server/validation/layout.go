package validation

import svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"

const (
	MaxLayoutNameLen        = 64
	MinLayoutAdminPINLength = 4
	MaxLayoutAdminPINLength = 8
)

// ValidateLayoutAdminPIN enforces the digit / length policy on a layout admin PIN (§7a.7).
func ValidateLayoutAdminPIN(pin string) error {
	if len(pin) < MinLayoutAdminPINLength || len(pin) > MaxLayoutAdminPINLength {
		return svcerrors.ErrLayoutAdminPINInvalid
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return svcerrors.ErrLayoutAdminPINInvalid
		}
	}
	return nil
}
