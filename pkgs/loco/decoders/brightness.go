package decoders

import (
	"fmt"
	"time"
)

// OutputBrightness holds the saved brightness state of a single output.
type OutputBrightness struct {
	Output  uint8
	CV      uint16
	Value   int
	Percent uint8
}

// BrightnessChangeable is implemented by decoders that support per-output brightness control.
type BrightnessChangeable interface {
	SetBrightness(output uint8, percent uint8) error
	GetBrightness(output uint8) (uint8, error)
	SetBrightnessRaw(output uint8, value int) error
	SnapshotBrightness() ([]OutputBrightness, error)
	Outputs() []uint8
}

const (
	brightnessTestBlinkPercent = 50
	brightnessTestBlinkCount   = 2
	brightnessTestStepDelay      = 300 * time.Millisecond
	brightnessTestOutputDelay    = 1 * time.Second
)

func validateBrightnessPercent(percent uint8) error {
	if percent > 100 {
		return fmt.Errorf("brightness must be between 0 and 100 percent, got %d", percent)
	}
	return nil
}

// RunBrightnessTest blinks each output twice (0% -> 50%), then restores saved values.
func RunBrightnessTest(decoder BrightnessChangeable, sleep func(time.Duration)) ([]OutputBrightness, error) {
	if sleep == nil {
		sleep = time.Sleep
	}

	snapshot, err := decoder.SnapshotBrightness()
	if err != nil {
		return nil, err
	}

	restoreAll := func() {
		for _, state := range snapshot {
			_ = decoder.SetBrightnessRaw(state.Output, state.Value)
		}
	}
	defer restoreAll()

	for _, state := range snapshot {
		for blink := 0; blink < brightnessTestBlinkCount; blink++ {
			if err := decoder.SetBrightness(state.Output, 0); err != nil {
				return snapshot, err
			}
			sleep(brightnessTestStepDelay)

			if err := decoder.SetBrightness(state.Output, brightnessTestBlinkPercent); err != nil {
				return snapshot, err
			}
			sleep(brightnessTestStepDelay)
		}

		if err := decoder.SetBrightnessRaw(state.Output, state.Value); err != nil {
			return snapshot, err
		}
		sleep(brightnessTestOutputDelay)
	}

	return snapshot, nil
}
