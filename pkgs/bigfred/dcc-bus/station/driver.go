// Package station builds a pkgs/loco/commandstation.Station for the
// daemon's command-station row. The parsing layer is intentionally
// permissive — operators paste a connection URI into the admin form
// and the daemon dials whatever it says.
package station

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/keskad/loco/pkgs/loco/commandstation"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// Open dials the command station described by cs. The returned
// Station is ready for SetSpeed / SetFn / ReadCV; the caller MUST
// call CleanUp on shutdown.
func Open(cs domain.CommandStation) (commandstation.Station, error) {
	switch cs.Kind {
	case domain.CommandStationKindZ21:
		host, port, err := parseHostPort(cs.ConnectionURI, "udp", 21105)
		if err != nil {
			return nil, fmt.Errorf("z21 uri %q: %w", cs.ConnectionURI, err)
		}
		return commandstation.NewZ21Roco(host, port)

	case domain.CommandStationKindLocoNetSerial:
		device, baud, err := parseSerial(cs.ConnectionURI)
		if err != nil {
			return nil, fmt.Errorf("loconet_serial uri %q: %w", cs.ConnectionURI, err)
		}
		return commandstation.NewLocoNetSerial(device, baud)

	case domain.CommandStationKindLocoNetTCP:
		// loconet_tcp speaks RAW binary LocoNet over TCP by default
		// (tcp://) — the common case (RocRail's lbtcp). The ASCII
		// LoconetOverTcp/LbServer protocol is selected with the lbserver://
		// scheme.
		uri := strings.TrimSpace(cs.ConnectionURI)
		if strings.HasPrefix(uri, "lbserver://") {
			host, port, err := parseHostPort(uri, "lbserver", 1234)
			if err != nil {
				return nil, fmt.Errorf("loconet_tcp uri %q: %w", cs.ConnectionURI, err)
			}
			return commandstation.NewLocoNetTCP(host, port)
		}
		host, port, err := parseHostPort(uri, "tcp", 1234)
		if err != nil {
			return nil, fmt.Errorf("loconet_tcp uri %q: %w", cs.ConnectionURI, err)
		}
		return commandstation.NewLocoNetTCPBinary(host, port)

	default:
		return nil, fmt.Errorf("unsupported command station kind %q", cs.Kind)
	}
}

// parseHostPort accepts any of:
//
//   - host:port
//   - <scheme>://host:port
//   - host (port defaults)
//
// It rejects empty inputs and ports outside the uint16 range.
func parseHostPort(uri, scheme string, defaultPort uint16) (string, uint16, error) {
	s := strings.TrimSpace(uri)
	if s == "" {
		return "", 0, fmt.Errorf("empty uri")
	}
	s = strings.TrimPrefix(s, scheme+"://")
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		// no port → use default
		return s, defaultPort, nil
	}
	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port: %w", err)
	}
	return host, uint16(p), nil
}

// parseSerial accepts:
//
//   - serial:///dev/ttyUSB0:57600
//   - /dev/ttyUSB0:57600
//   - /dev/ttyUSB0          (baud defaults to 57600)
func parseSerial(uri string) (string, int, error) {
	s := strings.TrimSpace(uri)
	if s == "" {
		return "", 0, fmt.Errorf("empty uri")
	}
	s = strings.TrimPrefix(s, "serial://")
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return s, 57600, nil
	}
	device := s[:idx]
	baud, err := strconv.Atoi(s[idx+1:])
	if err != nil {
		return s, 57600, nil
	}
	return device, baud, nil
}
