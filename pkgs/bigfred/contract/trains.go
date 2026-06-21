package contract

import "math"

// TrainSetSpeedWire is the inner payload of a train.setSpeed frame on
// the dcc-bus WebSocket.
type TrainSetSpeedWire struct {
	TrainID string   `json:"trainId"`
	Speed   uint8  `json:"speed"`
	Forward bool   `json:"forward"`
}

// LeadingMember returns the first member with a DCC address in Position
// order, plus whether one was found.
func (t DefinedTrain) LeadingMember() (DefinedTrainMember, bool) {
	for _, m := range t.Members {
		if m.Addr != nil && !m.ExcludeFromSpeed {
			return m, true
		}
	}
	return DefinedTrainMember{}, false
}

// IsLeading reports whether m is the same consist row as leading.
func (m DefinedTrainMember) IsLeading(leading DefinedTrainMember) bool {
	return m.VehicleID == leading.VehicleID && m.Position == leading.Position
}

// CanDrive reports whether userID may drive this train using the
// published controllerUserIds snapshot.
func (t DefinedTrain) CanDrive(userID uint) bool {
	for _, id := range t.ControllerUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// EffectiveMemberSpeed computes the DCC speed step for one powered
// member given the leading vehicle's target speed and the max step.
func EffectiveMemberSpeed(leadingSpeed uint8, multiplier float64, maxSpeed uint8) uint8 {
	if leadingSpeed == 0 {
		return 0
	}
	if multiplier <= 0 {
		multiplier = 1.0
	}
	raw := int(math.Round(float64(leadingSpeed) * multiplier))
	if raw < 0 {
		raw = 0
	}
	if raw > int(maxSpeed) {
		raw = int(maxSpeed)
	}
	return uint8(raw)
}

// MaxSpeedForSpeedSteps maps catalogue speed_steps to the top DCC step.
func MaxSpeedForSpeedSteps(speedSteps uint) uint8 {
	switch speedSteps {
	case 14:
		return 15
	case 28:
		return 28
	default:
		return 127
	}
}
