package validation

import (
	"strings"

	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const (
	MaxTrainNameLen        = 64
	DefaultSpeedMultiplier = 1.0
	MaxSpeedMultiplier     = 4.0
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
