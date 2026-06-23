package contract

// ClampSpeedForControllerLimit caps a throttle speed step when the
// driving user has a lease speed limit expressed as a percent of
// maxSpeed (1–100). limitPercent 0 or ≥100 means no cap.
func ClampSpeedForControllerLimit(speed, maxSpeed, limitPercent uint8) uint8 {
	if limitPercent == 0 || limitPercent >= 100 || maxSpeed == 0 {
		return speed
	}
	cap := uint8((uint16(maxSpeed)*uint16(limitPercent) + 50) / 100)
	if cap < 2 {
		cap = 2
	}
	if speed > cap {
		return cap
	}
	return speed
}
