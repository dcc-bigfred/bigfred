package decoders

import "fmt"

// CVAccess abstracts read/write of decoder CVs over a command station.
type CVAccess interface {
	ReadCV(num uint16) (int, error)
	WriteCV(num uint16, value int) error
}

// VolumeChangeable is implemented by decoders that support master volume control.
type VolumeChangeable interface {
	SetVolume(percent uint8) error
	GetVolume() (uint8, error)
}

func validatePercent(percent uint8) error {
	if percent > 100 {
		return fmt.Errorf("volume must be between 0 and 100 percent, got %d", percent)
	}
	return nil
}

func percentToCV(percent, maxCV uint8) int {
	return int((uint16(percent)*uint16(maxCV) + 50) / 100)
}

func cvToPercent(cv, maxCV int) uint8 {
	if maxCV <= 0 {
		return 0
	}
	return uint8((uint16(cv)*100 + uint16(maxCV)/2) / uint16(maxCV))
}
