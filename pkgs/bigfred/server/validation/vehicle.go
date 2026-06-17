package validation

import (
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const (
	MaxVehicleNameLen   = 64
	MaxVehicleNumberLen = 32
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
