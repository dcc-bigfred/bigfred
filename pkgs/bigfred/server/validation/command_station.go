package validation

import (
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

const maxCommandStationNameLen = 64

var validCommandStationSpeedSteps = map[uint]struct{}{
	14:  {},
	28:  {},
	128: {},
}

func SanitiseCommandStationInput(
	name string,
	kind domain.CommandStationKind,
	uri string,
	speedSteps uint,
) (string, domain.CommandStationKind, string, uint, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxCommandStationNameLen {
		return "", "", "", 0, svcerrors.ErrCommandStationNameRequired
	}
	if !kind.IsValid() {
		return "", "", "", 0, svcerrors.ErrCommandStationKindInvalid
	}
	if speedSteps == 0 {
		speedSteps = 128
	}
	if _, ok := validCommandStationSpeedSteps[speedSteps]; !ok {
		return "", "", "", 0, svcerrors.ErrCommandStationSpeedInvalid
	}
	return name, kind, strings.TrimSpace(uri), speedSteps, nil
}

func SanitiseCommandStationName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxCommandStationNameLen {
		return "", svcerrors.ErrCommandStationNameRequired
	}
	return name, nil
}

func ValidateCommandStationSpeedSteps(speedSteps uint) error {
	if _, ok := validCommandStationSpeedSteps[speedSteps]; !ok {
		return svcerrors.ErrCommandStationSpeedInvalid
	}
	return nil
}
