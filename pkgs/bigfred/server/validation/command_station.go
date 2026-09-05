package validation

import (
	"net"
	"strconv"
	"strings"

	"github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/domain"
	svcerrors "github.com/dcc-bigfred/bigfred/pkgs/bigfred/server/errors"
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

const (
	maxCommandStationIdleTimeoutSecs = 3600
)

// SanitiseCommandStationMaxLoconetSlots normalises the BigFred LocoNet slot budget.
// Zero selects the catalogue default (80).
func SanitiseCommandStationMaxLoconetSlots(maxSlots uint) (uint, error) {
	if maxSlots == 0 {
		return 0, nil
	}
	if maxSlots > domain.MaxLocoNetPhysicalSlots-1 {
		return 0, svcerrors.ErrCommandStationMaxLoconetSlotsInvalid
	}
	return maxSlots, nil
}

// SanitiseCommandStationIdleTimeoutSecs validates the remote idle window.
// Zero means disabled.
func SanitiseCommandStationIdleTimeoutSecs(secs uint) error {
	if secs > maxCommandStationIdleTimeoutSecs {
		return svcerrors.ErrCommandStationIdleTimeoutInvalid
	}
	return nil
}

// SanitiseCommandStationProgrammingTrackOutput normalises the default
// programming track output. Empty selects the catalogue default ("prog").
func SanitiseCommandStationProgrammingTrackOutput(track string) (string, error) {
	track = strings.ToLower(strings.TrimSpace(track))
	if track == "" {
		return domain.DefaultCommandStationProgrammingTrackOutput, nil
	}
	if !domain.IsValidProgrammingTrackOutput(track) {
		return "", svcerrors.ErrCommandStationProgrammingTrackInvalid
	}
	return track, nil
}

// ValidateWithrottlePortConflict rejects a WiThrottle client that would
// dial this process's own inbound WiThrottle listener (loopback + same port).
func ValidateWithrottlePortConflict(kind domain.CommandStationKind, uri string, serverEnabled bool, inboundPort uint16) error {
	if kind != domain.CommandStationKindWiThrottle || !serverEnabled {
		return nil
	}
	host, port, err := parseCommandStationHostPort(uri, "withrottle", domain.DefaultWithrottleInboundPort)
	if err != nil {
		return nil
	}
	if port != inboundPort || !isLoopbackHost(host) {
		return nil
	}
	return svcerrors.ErrCommandStationWithrottlePortConflict
}

func parseCommandStationHostPort(uri, scheme string, defaultPort uint16) (string, uint16, error) {
	s := strings.TrimSpace(uri)
	if s == "" {
		return "", 0, strconv.ErrSyntax
	}
	for _, prefix := range []string{scheme + "://", "z21://", "udp://", "loconet-tcp://", "tcp://", "withrottle://", "lbserver://"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return s, defaultPort, nil
	}
	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, err
	}
	return host, uint16(p), nil
}

func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	switch h {
	case "", "localhost", "127.0.0.1", "::1", "0.0.0.0", "*":
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
