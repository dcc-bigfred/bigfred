package withrottle

import (
	"fmt"
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
