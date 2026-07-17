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
	FlagStationName   = "station-name"
	FlagStationKind   = "station-kind"
	FlagStationURI    = "station-uri"
	FlagSpeedSteps    = "speed-steps"
	FlagHeartbeatSecs  = "heartbeat-secs"
	FlagDeadmanSecs    = "deadman-secs"
	FlagPollIntervalMs = "poll-interval-ms"
	FlagMaxVehiclesPerUser = "max-vehicles-per-user"
	FlagMaxLoconetSlots    = "max-loconet-slots"
	FlagIdleTimeoutSecs    = "idle-timeout-secs"
	FlagBootStopEnabled    = "enable-boot-stop"
	FlagSingleVehicleControl = "enable-single-vehicle-control"
	FlagAllocatePhysicalSlots = "allocate-physical-slots"
)

// AppendStationFlags appends command-station connection flags for cs.
func AppendStationFlags(args []string, cs domain.CommandStation) []string {
	return append(args,
		"--"+FlagStationName, cs.Name,
		"--"+FlagStationKind, string(cs.Kind),
		"--"+FlagStationURI, cs.ConnectionURI,
		"--"+FlagSpeedSteps, strconv.FormatUint(uint64(cs.EffectiveSpeedSteps()), 10),
		"--"+FlagHeartbeatSecs, strconv.FormatFloat(cs.EffectiveHeartbeatSecs(), 'f', -1, 64),
		"--"+FlagDeadmanSecs, strconv.FormatFloat(cs.EffectiveDeadmanSecs(), 'f', -1, 64),
		"--"+FlagPollIntervalMs, strconv.FormatUint(uint64(cs.EffectivePollIntervalMs()), 10),
	)
}

// AppendLeaseFlags appends slot-lease limits for the daemon router.
func AppendLeaseFlags(args []string, layout domain.Layout, cs domain.CommandStation) []string {
	args = append(args,
		"--"+FlagMaxVehiclesPerUser, strconv.FormatUint(uint64(layout.EffectiveMaxVehiclesPerUser()), 10),
		"--"+FlagIdleTimeoutSecs, strconv.FormatUint(uint64(cs.IdleTimeoutSecs), 10),
	)
	if cs.Kind.IsLocoNet() {
		args = append(args,
			"--"+FlagMaxLoconetSlots, strconv.FormatUint(uint64(cs.EffectiveMaxLoconetSlots()), 10),
			"--"+FlagAllocatePhysicalSlots+"="+strconv.FormatBool(cs.EffectiveAllocatePhysicalSlots()),
		)
	}
	return args
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
