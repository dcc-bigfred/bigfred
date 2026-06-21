package decoders

import "fmt"

const (
	lokSoundV5VolumeCV         uint16 = 63
	lokSoundV5VolumeMaxCV      uint8  = 192 // 192 = 100%
	lokSoundV5BrightnessMaxCV  uint8  = 31  // 31 = 100%
	lokSoundV5Aux1BrightnessCV uint16 = 281
	lokSoundV5AuxOutputCount   uint8  = 18
	lokSoundV5IndexCV          uint16 = 31
	lokSoundV5IndexPageCV      uint16 = 32
	lokSoundV5IndexPageValue   int    = 16
	lokSoundV5OutputConfigPage int    = 0
)

// TODO: verify AUX brightness CV numbers against LokSound 5 manual p.85.
// Derived from AUX4 example (CV #302) with 7 CVs per output block.

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

func (d *LokSoundv5) Outputs() []uint8 {
	outputs := make([]uint8, lokSoundV5AuxOutputCount)
	for i := range outputs {
		outputs[i] = uint8(i + 1)
	}
	return outputs
}

func (d *LokSoundv5) selectOutputConfigPage() error {
	if err := d.cv.WriteCV(lokSoundV5IndexCV, lokSoundV5IndexPageValue); err != nil {
		return fmt.Errorf("failed to select CV index page: %w", err)
	}
	if err := d.cv.WriteCV(lokSoundV5IndexPageCV, lokSoundV5OutputConfigPage); err != nil {
		return fmt.Errorf("failed to select output config page: %w", err)
	}
	return nil
}

func (d *LokSoundv5) brightnessCVForOutput(output uint8) (uint16, error) {
	if output < 1 || output > lokSoundV5AuxOutputCount {
		return 0, fmt.Errorf("output %d is out of range (valid: 1-%d)", output, lokSoundV5AuxOutputCount)
	}
	return lokSoundV5Aux1BrightnessCV + uint16(output-1)*7, nil
}

func (d *LokSoundv5) SetBrightness(output uint8, percent uint8) error {
	if err := validateBrightnessPercent(percent); err != nil {
		return err
	}
	cv, err := d.brightnessCVForOutput(output)
	if err != nil {
		return err
	}
	if err := d.selectOutputConfigPage(); err != nil {
		return err
	}
	return d.cv.WriteCV(cv, percentToCV(percent, lokSoundV5BrightnessMaxCV))
}

func (d *LokSoundv5) GetBrightness(output uint8) (uint8, error) {
	cv, err := d.brightnessCVForOutput(output)
	if err != nil {
		return 0, err
	}
	if err := d.selectOutputConfigPage(); err != nil {
		return 0, err
	}
	value, err := d.cv.ReadCV(cv)
	if err != nil {
		return 0, fmt.Errorf("failed to read CV%d (output %d brightness): %w", cv, output, err)
	}
	return cvToPercent(value, int(lokSoundV5BrightnessMaxCV)), nil
}

func (d *LokSoundv5) SetBrightnessRaw(output uint8, value int) error {
	cv, err := d.brightnessCVForOutput(output)
	if err != nil {
		return err
	}
	if err := d.selectOutputConfigPage(); err != nil {
		return err
	}
	return d.cv.WriteCV(cv, value)
}

func (d *LokSoundv5) SnapshotBrightness() ([]OutputBrightness, error) {
	if err := d.selectOutputConfigPage(); err != nil {
		return nil, err
	}

	states := make([]OutputBrightness, 0, len(d.Outputs()))
	for _, output := range d.Outputs() {
		cv, err := d.brightnessCVForOutput(output)
		if err != nil {
			return nil, err
		}
		value, err := d.cv.ReadCV(cv)
		if err != nil {
			return nil, fmt.Errorf("failed to read CV%d (output %d brightness): %w", cv, output, err)
		}
		states = append(states, OutputBrightness{
			Output:  output,
			CV:      cv,
			Value:   value,
			Percent: cvToPercent(value, int(lokSoundV5BrightnessMaxCV)),
		})
	}
	return states, nil
}
