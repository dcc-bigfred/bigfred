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
		speedSteps = domain.DefaultCommandStationSpeedSteps
	}
	if _, ok := validCommandStationSpeedSteps[speedSteps]; !ok {
		return "", "", "", 0, svcerrors.ErrCommandStationSpeedInvalid
	}
	return name, kind, strings.TrimSpace(uri), speedSteps, nil
}

// SanitiseCommandStationSpeedSteps normalises the DCC speed-step count.
// Zero selects the catalogue default (128).
func SanitiseCommandStationSpeedSteps(speedSteps uint) (uint, error) {
	if speedSteps == 0 {
		speedSteps = domain.DefaultCommandStationSpeedSteps
	}
	if _, ok := validCommandStationSpeedSteps[speedSteps]; !ok {
		return 0, svcerrors.ErrCommandStationSpeedInvalid
	}
	return speedSteps, nil
}

func SanitiseCommandStationName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxCommandStationNameLen {
		return "", svcerrors.ErrCommandStationNameRequired
	}
	return name, nil
}

func ValidateCommandStationSpeedSteps(speedSteps uint) error {
	_, err := SanitiseCommandStationSpeedSteps(speedSteps)
	return err
}

const maxCommandStationPollIntervalMs = 60000

// SanitiseCommandStationPollInterval normalises the state-feed poll cadence.
// Zero selects the dcc-bus daemon default (750 ms).
func SanitiseCommandStationPollInterval(pollIntervalMs uint) (uint, error) {
	if pollIntervalMs > maxCommandStationPollIntervalMs {
		return 0, svcerrors.ErrCommandStationPollIntervalInvalid
	}
	return pollIntervalMs, nil
}

const (
	minCommandStationHeartbeatSecs = 1.0
	maxCommandStationHeartbeatSecs = 60.0
	minCommandStationDeadmanSecs   = 3.0
	maxCommandStationDeadmanSecs   = 120.0
)

// SanitiseCommandStationTiming normalises WS ping and dead-man windows for a
// command station. Zero values select catalogue defaults (2s / 6s).
func SanitiseCommandStationTiming(heartbeatSecs, deadmanSecs float64) (float64, float64, error) {
	if heartbeatSecs <= 0 {
		heartbeatSecs = domain.DefaultCommandStationHeartbeatSecs
	}
	if deadmanSecs <= 0 {
		deadmanSecs = domain.DefaultCommandStationDeadmanSecs
	}
	if heartbeatSecs < minCommandStationHeartbeatSecs || heartbeatSecs > maxCommandStationHeartbeatSecs {
		return 0, 0, svcerrors.ErrCommandStationHeartbeatInvalid
	}
	if deadmanSecs < minCommandStationDeadmanSecs || deadmanSecs > maxCommandStationDeadmanSecs {
		return 0, 0, svcerrors.ErrCommandStationDeadmanInvalid
	}
	if deadmanSecs <= heartbeatSecs {
		return 0, 0, svcerrors.ErrCommandStationDeadmanTooShort
	}
	return heartbeatSecs, deadmanSecs, nil
}
