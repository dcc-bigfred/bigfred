package contract

import (
	"fmt"
	"math/rand"
	"strings"
	"unicode"
)

const (
	WithrottlePairingCodeLen      = 6
	DefaultWithrottleInboundPort  = 12090
	DefaultWithrottlePairingAddr  = 3
	DefaultWithrottleHeartbeatSecs = 10
)

// WithrottlePairLabel formats a 6-digit code for the reqdedup SET.
func WithrottlePairLabel(code string) string {
	return code
}

// WithrottlePairingDisplayLabel is shorthand shown in the UI (e.g. "1 2 2 1 4 5").
func WithrottlePairingDisplayLabel(code string) string {
	if len(code) == 0 {
		return ""
	}
	var b strings.Builder
	for i, ch := range code {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// WithrottlePairReqID formats the WiThrottle pending-req identifier (e.g. "withrottle:122145").
func WithrottlePairReqID(code string) string {
	return RemoteProtocolWithrottle + ":" + code
}

// ValidWithrottleCode reports whether code is exactly six decimal digits.
func ValidWithrottleCode(code string) bool {
	if len(code) != WithrottlePairingCodeLen {
		return false
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// RandomPairingCode picks one 6-digit decimal code using rng.
func RandomPairingCode(rng *rand.Rand) string {
	if rng == nil {
		rng = rand.New(rand.NewSource(NowMS()))
	}
	var b strings.Builder
	b.Grow(WithrottlePairingCodeLen)
	for i := 0; i < WithrottlePairingCodeLen; i++ {
		b.WriteRune(rune('0' + rng.Intn(10)))
	}
	return b.String()
}

// NormalizeWithrottleDeviceName strips whitespace for N-code pairing match.
func NormalizeWithrottleDeviceName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, name)
}

// WithrottleSentinelRosterName is the display label for the pairing sentinel loco.
const WithrottleSentinelRosterName = "Pair with BigFred"

// FormatWithrottleSentinelRosterLine builds RL1 for unpaired clients.
func FormatWithrottleSentinelRosterLine(sentinelAddr uint16) string {
	addrType := "S"
	if sentinelAddr >= 128 {
		addrType = "L"
	}
	return fmt.Sprintf("RL1]\\[%s}|{%d}|{%s", WithrottleSentinelRosterName, sentinelAddr, addrType)
}
