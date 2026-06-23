package sim

import "time"

// Config tunes the sustained drive pattern.
type Config struct {
	MaxSpeed          uint8
	LegDuration       time.Duration
	WithoutF1         bool
	HornFunction      uint8
	HornMinInterval   time.Duration
	HornMaxInterval   time.Duration
	HornPulseDuration time.Duration
}
