package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// Station flag names — keep in sync with loco-server supervisord spawn
// (pkgs/bigfred/server/service/dcc_bus.go).
const (
	FlagStationName = "station-name"
	FlagStationKind = "station-kind"
	FlagStationURI  = "station-uri"
	FlagSpeedSteps  = "speed-steps"
)

// AppendStationFlags appends command-station connection flags for cs.
func AppendStationFlags(args []string, cs domain.CommandStation) []string {
	steps := cs.SpeedSteps
	if steps == 0 {
		steps = 128
	}
	return append(args,
		"--"+FlagStationName, cs.Name,
		"--"+FlagStationKind, string(cs.Kind),
		"--"+FlagStationURI, cs.ConnectionURI,
		"--"+FlagSpeedSteps, strconv.FormatUint(uint64(steps), 10),
	)
}

// CommandStationFromFlags parses CLI flags into a CommandStation row
// shape for daemon boot. ID comes from --command-station-id.
func CommandStationFromFlags(id uint, name, kind, uri string, speedSteps uint) (domain.CommandStation, error) {
	if id == 0 {
		return domain.CommandStation{}, errors.New("command-station-id is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.CommandStation{}, errors.New("station-name is required")
	}
	k := domain.CommandStationKind(strings.TrimSpace(kind))
	if !k.IsValid() {
		return domain.CommandStation{}, fmt.Errorf("unsupported station-kind %q", kind)
	}
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return domain.CommandStation{}, errors.New("station-uri is required")
	}
	if speedSteps == 0 {
		speedSteps = 128
	}
	return domain.CommandStation{
		ID:            id,
		Name:          name,
		Kind:          k,
		ConnectionURI: uri,
		SpeedSteps:    speedSteps,
	}, nil
}
