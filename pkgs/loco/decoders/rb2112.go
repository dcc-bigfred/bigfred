package decoders

import "fmt"

const (
	railboxRB2112BrightnessMaxCV    uint8  = 255
	railboxRB2112BrightnessOutputMax uint8  = 13
)

var railboxRB2112MaxBrightnessCVs = map[uint8]uint16{
	1: 41, 2: 42, 3: 43, 4: 44, 5: 45, 6: 46, 7: 47, 8: 48,
	9: 106, 10: 107, 11: 108, 12: 109, 13: 110,
}

var railboxRB2112Outputs = []uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}

// RailboxRB2112 implements BrightnessChangeable for RB 2112 wagon function decoders.
type RailboxRB2112 struct {
	cv CVAccess
}

func NewRailboxRB2112(cv CVAccess) *RailboxRB2112 {
	return &RailboxRB2112{cv: cv}
}

func (d *RailboxRB2112) Outputs() []uint8 {
	return append([]uint8(nil), railboxRB2112Outputs...)
}

func (d *RailboxRB2112) brightnessCVForOutput(output uint8) (uint16, error) {
	if d.cv == nil {
		return 0, fmt.Errorf("CV access not configured")
	}
	cv, ok := railboxRB2112MaxBrightnessCVs[output]
	if !ok {
		return 0, fmt.Errorf("output %d is out of range (valid: 1-%d)", output, railboxRB2112BrightnessOutputMax)
	}
	return cv, nil
}

func (d *RailboxRB2112) SetBrightness(output uint8, percent uint8) error {
	if err := validateBrightnessPercent(percent); err != nil {
		return err
	}
	cv, err := d.brightnessCVForOutput(output)
	if err != nil {
		return err
	}
	return d.cv.WriteCV(cv, percentToCV(percent, railboxRB2112BrightnessMaxCV))
}

func (d *RailboxRB2112) GetBrightness(output uint8) (uint8, error) {
	cv, err := d.brightnessCVForOutput(output)
	if err != nil {
		return 0, err
	}
	value, err := d.cv.ReadCV(cv)
	if err != nil {
		return 0, fmt.Errorf("failed to read CV%d (output %d brightness): %w", cv, output, err)
	}
	return cvToPercent(value, int(railboxRB2112BrightnessMaxCV)), nil
}

func (d *RailboxRB2112) SetBrightnessRaw(output uint8, value int) error {
	cv, err := d.brightnessCVForOutput(output)
	if err != nil {
		return err
	}
	return d.cv.WriteCV(cv, value)
}

func (d *RailboxRB2112) SnapshotBrightness() ([]OutputBrightness, error) {
	states := make([]OutputBrightness, 0, len(railboxRB2112Outputs))
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
			Percent: cvToPercent(value, int(railboxRB2112BrightnessMaxCV)),
		})
	}
	return states, nil
}
