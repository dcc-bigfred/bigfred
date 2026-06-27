package contract

import (
	"fmt"
	"math/rand"
)

const (
	Z21PairingActiveKeyScanTmpl = "bigfred:remote:active:%d:%d:*"
)

// Z21PairLabel formats a CV3/CV4 pair for the reqpairs SET.
func Z21PairLabel(pairingCV3, pairingCV4 int) string {
	return fmt.Sprintf("%d:%d", pairingCV3, pairingCV4)
}

// Z21PairingDisplayLabel is shorthand shown in the UI (e.g. "122-145").
func Z21PairingDisplayLabel(pairingCV3, pairingCV4 int) string {
	return fmt.Sprintf("%d-%d", pairingCV3, pairingCV4)
}

// Z21PairReqID formats the Z21 pending-req identifier (e.g. "z21:122:145").
func Z21PairReqID(pairingCV3, pairingCV4 int) string {
	return RemoteProtocolZ21 + ":" + Z21PairLabel(pairingCV3, pairingCV4)
}

// Z21PairingActiveKeyScanPattern matches every active session on one CS.
func Z21PairingActiveKeyScanPattern(layoutID, commandStationID uint) string {
	return fmt.Sprintf(Z21PairingActiveKeyScanTmpl, layoutID, commandStationID)
}

// ValidPairingCV reports whether v satisfies the [1–2][1–5][1–5] pattern (111–255).
func ValidPairingCV(v int) bool {
	if v < 111 || v > 255 {
		return false
	}
	d1, d2, d3 := v/100, (v/10)%10, v%10
	return d1 >= 1 && d1 <= 2 && d2 >= 1 && d2 <= 5 && d3 >= 1 && d3 <= 5
}

// AllValidPairingCVs returns every allowed single-CV pairing value.
func AllValidPairingCVs() []int {
	out := make([]int, 0, 50)
	for v := 111; v <= 255; v++ {
		if ValidPairingCV(v) {
			out = append(out, v)
		}
	}
	return out
}

var allValidPairingCVs = AllValidPairingCVs()

// RandomPairingCV picks one valid CV value using rng.
func RandomPairingCV(rng *rand.Rand) int {
	return allValidPairingCVs[rng.Intn(len(allValidPairingCVs))]
}

// ValidHandsetBrakeSecs reports whether secs is in the allowed pairing range.
func ValidHandsetBrakeSecs(secs uint) bool {
	return secs >= Z21HandsetBrakeSecsMin && secs <= Z21HandsetBrakeSecsMax
}

// NormaliseHandsetBrakeSecs returns secs clamped to the allowed range, or the
// default when secs is zero (legacy rows).
func NormaliseHandsetBrakeSecs(secs uint) uint {
	if secs == 0 {
		return Z21HandsetBrakeSecsDefault
	}
	if secs < Z21HandsetBrakeSecsMin {
		return Z21HandsetBrakeSecsMin
	}
	if secs > Z21HandsetBrakeSecsMax {
		return Z21HandsetBrakeSecsMax
	}
	return secs
}

const (
	Z21HandsetBrakeSecsDefault = 6
	Z21HandsetBrakeSecsMin     = 6
	Z21HandsetBrakeSecsMax     = 60
)
