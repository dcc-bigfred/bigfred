package decoders

import "fmt"

const (
	zimoMS450VolumeCV    uint16 = 266
	zimoMS450VolumeMaxCV uint8  = 65 // 65 = 100%
)

// ZIMOMS450 implements VolumeChangeable for ZIMO MS/MN sound decoders.
type ZIMOMS450 struct {
	cv CVAccess
}

func NewZIMOMS450(cv CVAccess) *ZIMOMS450 {
	return &ZIMOMS450{cv: cv}
}

func (d *ZIMOMS450) SetVolume(percent uint8) error {
	if err := validatePercent(percent); err != nil {
		return err
	}
	return d.cv.WriteCV(zimoMS450VolumeCV, percentToCV(percent, zimoMS450VolumeMaxCV))
}

func (d *ZIMOMS450) GetVolume() (uint8, error) {
	cv, err := d.cv.ReadCV(zimoMS450VolumeCV)
	if err != nil {
		return 0, fmt.Errorf("failed to read CV%d (master volume): %w", zimoMS450VolumeCV, err)
	}
	return cvToPercent(cv, int(zimoMS450VolumeMaxCV)), nil
}
