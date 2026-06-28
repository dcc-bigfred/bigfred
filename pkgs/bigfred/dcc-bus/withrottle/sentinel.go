package withrottle

import (
	"fmt"
	"strconv"
	"strings"
)

// isSentinelAddr reports whether addr is the configured pairing sentinel.
func isSentinelAddr(addr, sentinel uint16) bool {
	return addr != 0 && addr == sentinel
}

// allowUnpairedAcquire reports whether an unpaired client may acquire addr.
func allowUnpairedAcquire(addr, sentinel uint16, paired bool) bool {
	return !paired && isSentinelAddr(addr, sentinel)
}

// buildSentinelAcquireReply is the acquire reply for the pairing sentinel.
// It advertises F0–F9 labels so Engine Driver shows named function buttons
// the user taps to enter the pairing code digits.
func buildSentinelAcquireReply(throttleID byte, addr uint16) []string {
	id := string(throttleID)
	key := locoKeyForAddr(addr)
	lines := []string{
		fmt.Sprintf("M%s+%s%s", id, key, propSep),
	}
	var labels strings.Builder
	labels.WriteString(fmt.Sprintf("M%sL%s%s", id, key, propSep))
	for fn := 0; fn <= 9; fn++ {
		labels.WriteString(entrySep)
		labels.WriteString(fmt.Sprintf("F%d", fn))
	}
	lines = append(lines, labels.String())
	for fn := 0; fn <= 9; fn++ {
		lines = append(lines, fmt.Sprintf("M%sA%s%sF0%d", id, key, propSep, fn))
	}
	lines = append(lines,
		fmt.Sprintf("M%sA%s%sV0", id, key, propSep),
		fmt.Sprintf("M%sA%s%sR1", id, key, propSep),
		fmt.Sprintf("M%sA%s%ss1", id, key, propSep),
	)
	return lines
}

// buildReleaseLine returns the M…- confirmation for one loco key.
func buildReleaseLine(throttleID byte, locoKey string) string {
	return "M" + string(throttleID) + "-" + locoKey + propSep
}

// buildSentinelReleaseLines releases the sentinel after pairing.
func buildSentinelReleaseLines(throttleID byte, sentinel uint16) []string {
	key := locoKeyForAddr(sentinel)
	return []string{buildReleaseLine(throttleID, key) + "r"}
}

// parseSpeedValue reads V payload into WiThrottle wire speed 0–126.
func parseSpeedValue(prop string) (wireSpeed int, estop bool, ok bool) {
	if len(prop) < 2 || prop[0] != 'V' {
		return 0, false, false
	}
	n, err := strconv.Atoi(prop[1:])
	if err != nil {
		return 0, false, false
	}
	if n < 0 {
		return 1, true, true
	}
	if n == 1 {
		return 1, true, true
	}
	if n > 126 {
		n = 126
	}
	return n, false, true
}

// dccSpeedFromWire maps WiThrottle V encoding to DCC 128-step speed.
func dccSpeedFromWire(wireSpeed int, speedSteps uint) uint8 {
	if wireSpeed <= 0 {
		return 0
	}
	if wireSpeed == 1 {
		return 0
	}
	if speedSteps == 0 {
		speedSteps = 128
	}
	max := int(speedSteps) - 1
	step := ((wireSpeed-1)*max + 62) / 125
	if step > max {
		step = max
	}
	if step < 0 {
		step = 0
	}
	return uint8(step)
}

// wireSpeedFromDCC maps DCC speed to WiThrottle V encoding.
func wireSpeedFromDCC(speed uint8, speedSteps uint) int {
	if speed == 0 {
		return 0
	}
	if speedSteps == 0 {
		speedSteps = 128
	}
	max := int(speedSteps) - 1
	if max <= 0 {
		return 2
	}
	wire := 1 + (int(speed)*125+max/2)/max
	if wire < 2 {
		wire = 2
	}
	if wire > 126 {
		wire = 126
	}
	return wire
}
