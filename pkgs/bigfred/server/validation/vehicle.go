package validation

import (
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const (
	MaxVehicleNameLen       = 64
	MaxVehicleNumberLen     = 32
	MaxVehicleCarrierLen    = 64
	MaxVehicleAssignmentLen = 128
)

// SanitiseVehicleName trims whitespace and enforces a non-empty name.
func SanitiseVehicleName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", svcerrors.ErrVehicleNameRequired
	}
	if len(name) > MaxVehicleNameLen {
		name = name[:MaxVehicleNameLen]
	}
	return name, nil
}

// TrimVehicleNumber trims and caps the optional vehicle number field.
func TrimVehicleNumber(raw string) string {
	number := strings.TrimSpace(raw)
	if len(number) > MaxVehicleNumberLen {
		number = number[:MaxVehicleNumberLen]
	}
	return number
}

// TrimVehicleCarrier trims and caps the optional carrier field.
func TrimVehicleCarrier(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) > MaxVehicleCarrierLen {
		s = s[:MaxVehicleCarrierLen]
	}
	return s
}

// TrimVehicleAssignment trims and caps the optional assignment field.
func TrimVehicleAssignment(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) > MaxVehicleAssignmentLen {
		s = s[:MaxVehicleAssignmentLen]
	}
	return s
}

// ParseVehicleEpoch validates an optional epoch code. Empty is allowed.
func ParseVehicleEpoch(raw string) (domain.VehicleEpoch, error) {
	e := domain.VehicleEpoch(strings.TrimSpace(raw))
	if !e.IsValid() {
		return "", svcerrors.ErrVehicleEpochInvalid
	}
	return e, nil
}

// ParseVehicleRevisionDate parses YYYY-MM-DD or empty/nil into a date-only UTC time.
func ParseVehicleRevisionDate(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, svcerrors.ErrVehicleRevisionDateInvalid
	}
	return &t, nil
}

// FormatVehicleRevisionDate formats a stored date as YYYY-MM-DD, or nil.
func FormatVehicleRevisionDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02")
	return &s
}

// ResolveVehicleDeadManFields applies defaults and validates optional
// dead-man and function indices on create.
func ResolveVehicleDeadManFields(
	rp1Fn, emergFn *uint8,
	dmsOpt *domain.DeadManSwitchOption,
) (uint8, uint8, domain.DeadManSwitchOption, error) {
	rp1 := domain.DefaultVehicleRp1Function
	if rp1Fn != nil {
		if !domain.IsValidDccFunctionNum(*rp1Fn) {
			return 0, 0, "", svcerrors.ErrVehicleDccFunctionInvalid
		}
		rp1 = *rp1Fn
	}
	emerg := domain.DefaultVehicleEmergencyLightsFunction
	if emergFn != nil {
		if !domain.IsValidDccFunctionNum(*emergFn) {
			return 0, 0, "", svcerrors.ErrVehicleDccFunctionInvalid
		}
		emerg = *emergFn
	}
	opt := domain.DeadManSwitchStop
	if dmsOpt != nil {
		if !dmsOpt.IsValid() {
			return 0, 0, "", svcerrors.ErrVehicleDeadManSwitchInvalid
		}
		opt = *dmsOpt
	}
	return rp1, emerg, opt, nil
}
