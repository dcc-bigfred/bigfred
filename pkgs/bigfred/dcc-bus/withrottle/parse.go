package withrottle

import (
	"strconv"

	wtproto "github.com/dcc-bigfred/proto/go/pkgs/withrottle"
)

const (
	propSep    = "<;>"
	entrySep   = `]\[`
	segmentSep = "}|{"
)

const maxWiThrottleFunction = 31

func parseLocoKey(s string) (addr uint16, isLong bool, ok bool) {
	return wtproto.ParseLocoKey(s)
}

func locoKeyForAddr(addr uint16) string {
	return wtproto.LocoKey(addr)
}

func dccSpeedFromWire(wireSpeed int, speedSteps uint) uint8 {
	return wtproto.DccSpeedFromWire(wireSpeed, speedSteps)
}

func wireSpeedFromDCC(speed uint8, speedSteps uint) int {
	return wtproto.WireSpeedFromDCC(speed, speedSteps)
}

func parseSpeedValue(prop string) (wireSpeed int, estop bool, ok bool) {
	return wtproto.ParseSpeedValue(prop)
}

func parseMAction(line string) (wtproto.MCommand, bool) {
	return wtproto.ParseM(line)
}

func parseFunctionAction(prop string) (fn int, on bool, force bool, ok bool) {
	if len(prop) < 2 {
		return 0, false, false, false
	}
	switch prop[0] {
	case 'F':
		force = false
	case 'f':
		force = true
	default:
		return 0, false, false, false
	}
	if prop[1] != '0' && prop[1] != '1' {
		return 0, false, false, false
	}
	on = prop[1] == '1'
	n, err := strconv.Atoi(prop[2:])
	if err != nil || n < 0 || n > maxWiThrottleFunction {
		return 0, false, false, false
	}
	return n, on, force, true
}
