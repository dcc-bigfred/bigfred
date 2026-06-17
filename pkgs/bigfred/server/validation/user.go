package validation

import (
	"strings"
	"unicode"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const (
	MinUserPINLength = 4
	MaxUserPINLength = 12
	MaxUserLoginLen  = 32
)

// SanitiseLogin trims and validates the ASCII-only login used in admin paths.
func SanitiseLogin(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", svcerrors.ErrUserLoginRequired
	}
	if len(trimmed) > MaxUserLoginLen {
		return "", svcerrors.ErrUserLoginInvalid
	}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_':
		default:
			return "", svcerrors.ErrUserLoginInvalid
		}
	}
	return trimmed, nil
}

// ValidateUserPIN enforces the all-digit + length contract for user PINs.
func ValidateUserPIN(pin string) error {
	if pin == "" {
		return svcerrors.ErrUserPINRequired
	}
	if len(pin) < MinUserPINLength || len(pin) > MaxUserPINLength {
		return svcerrors.ErrUserPINInvalid
	}
	for _, r := range pin {
		if !unicode.IsDigit(r) {
			return svcerrors.ErrUserPINInvalid
		}
	}
	return nil
}

// IsPermanentRole gates the closed catalogue of permanent roles.
func IsPermanentRole(r domain.Role) bool {
	switch r {
	case domain.RoleDriver, domain.RoleAdmin:
		return true
	default:
		return false
	}
}
