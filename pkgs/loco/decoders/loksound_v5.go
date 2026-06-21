package decoders

import "fmt"

const (
	lokSoundV5VolumeCV    uint16 = 63
	lokSoundV5VolumeMaxCV uint8  = 192 // 192 = 100%
)

// LokSoundv5 implements VolumeChangeable for ESU LokSound 5 decoders.
type LokSoundv5 struct {
	cv CVAccess
}

func NewLokSoundv5(cv CVAccess) *LokSoundv5 {
	return &LokSoundv5{cv: cv}
}

func (d *LokSoundv5) SetVolume(percent uint8) error {
	if err := validatePercent(percent); err != nil {
		return err
	}
	return d.cv.WriteCV(lokSoundV5VolumeCV, percentToCV(percent, lokSoundV5VolumeMaxCV))
}

func (d *LokSoundv5) GetVolume() (uint8, error) {
	cv, err := d.cv.ReadCV(lokSoundV5VolumeCV)
	if err != nil {
		return 0, fmt.Errorf("failed to read CV%d (master volume): %w", lokSoundV5VolumeCV, err)
	}
	return cvToPercent(cv, int(lokSoundV5VolumeMaxCV)), nil
}
