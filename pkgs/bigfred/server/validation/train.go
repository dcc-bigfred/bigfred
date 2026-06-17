package validation

import (
	"strings"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const (
	MaxTrainNameLen        = 64
	DefaultSpeedMultiplier = 1.0
	MaxSpeedMultiplier     = 4.0
	MinStartDelayMs        = 50
	MaxStartDelayMs        = 1000
	StartDelayStepMs       = 50
	MaxAccelRampMs           = 5000
	AccelRampStepMs          = 500
	MaxAccelRampMaxSteps     = 10
	DefaultAccelRampMaxSteps = 1
	MaxBrakeRampMs           = MaxAccelRampMs
	BrakeRampStepMs          = AccelRampStepMs
	MaxBrakeRampMaxSteps     = MaxAccelRampMaxSteps
	DefaultBrakeRampMaxSteps = DefaultAccelRampMaxSteps
)

// SanitiseTrainName trims whitespace and enforces a non-empty name.
func SanitiseTrainName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", svcerrors.ErrTrainNameRequired
	}
	if len(name) > MaxTrainNameLen {
		name = name[:MaxTrainNameLen]
	}
	return name, nil
}

// ValidateSpeedMultiplier checks a train member speed multiplier.
func ValidateSpeedMultiplier(v float64) error {
	if v <= 0 || v > MaxSpeedMultiplier {
		return svcerrors.ErrTrainMemberMultiplierRange
	}
	return nil
}

// ValidateStartDelayMs checks a train member start delay (0 or 50–1000 ms, step 50).
func ValidateStartDelayMs(ms int) error {
	if ms == 0 {
		return nil
	}
	if ms < MinStartDelayMs || ms > MaxStartDelayMs || ms%StartDelayStepMs != 0 {
		return svcerrors.ErrTrainMemberStartDelayRange
	}
	return nil
}

// ValidateAccelRampMs checks consist acceleration ramp duration (0 or 500–5000 ms, step 500).
func ValidateAccelRampMs(ms int) error {
	if ms == 0 {
		return nil
	}
	if ms < AccelRampStepMs || ms > MaxAccelRampMs || ms%AccelRampStepMs != 0 {
		return svcerrors.ErrTrainMemberAccelRampRange
	}
	return nil
}

// ValidateAccelRampMaxSteps checks the configured maximum ramp step count.
func ValidateAccelRampMaxSteps(steps int) error {
	if steps < 1 || steps > MaxAccelRampMaxSteps {
		return svcerrors.ErrTrainMemberAccelRampStepsRange
	}
	return nil
}

// ValidateBrakeRampMs checks consist braking ramp duration (0 or 500–5000 ms, step 500).
func ValidateBrakeRampMs(ms int) error {
	if ms == 0 {
		return nil
	}
	if ms < BrakeRampStepMs || ms > MaxBrakeRampMs || ms%BrakeRampStepMs != 0 {
		return svcerrors.ErrTrainMemberBrakeRampRange
	}
	return nil
}

// ValidateBrakeRampMaxSteps checks the configured maximum braking ramp step count.
func ValidateBrakeRampMaxSteps(steps int) error {
	if steps < 1 || steps > MaxBrakeRampMaxSteps {
		return svcerrors.ErrTrainMemberBrakeRampStepsRange
	}
	return nil
}
