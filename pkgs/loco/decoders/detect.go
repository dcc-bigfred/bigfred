package decoders

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

const (
	ManufacturerZIMO    = 145
	ManufacturerESU     = 151
	ManufacturerRailBOX = 172
)

// DecoderKind identifies a supported decoder family.
type DecoderKind int

const (
	DecoderUnknown DecoderKind = iota
	DecoderRailBOX
	DecoderESU
	DecoderZIMO
)

// Identification holds decoder identity read from CV7 and CV8.
type Identification struct {
	Kind            DecoderKind
	Name            string
	ManufacturerID  int // CV8
	SoftwareVersion int // CV7; -1 when CV7 could not be read
}

// Identify reads CV7 and CV8 and returns decoder identity.
func Identify(cv CVAccess) (Identification, error) {
	cv8, err := cv.ReadCV(8)
	if err != nil {
		return Identification{}, fmt.Errorf("failed to read CV8 (manufacturer ID): %w", err)
	}

	id := Identification{ManufacturerID: cv8, SoftwareVersion: -1}

	cv7, cv7Err := cv.ReadCV(7)
	if cv7Err != nil {
		logrus.Debugf("failed to read CV7 (software version): %v", cv7Err)
	} else {
		id.SoftwareVersion = cv7
		logrus.Debugf("decoder CV7=%d CV8=%d", cv7, cv8)
	}

	switch cv8 {
	case ManufacturerRailBOX:
		id.Kind = DecoderRailBOX
		id.Name = "RailBOX RB23xx"
	case ManufacturerESU:
		id.Kind = DecoderESU
		id.Name = "ESU LokSound 5"
	case ManufacturerZIMO:
		id.Kind = DecoderZIMO
		id.Name = "ZIMO MS/MN"
	default:
		id.Kind = DecoderUnknown
		id.Name = "unknown"
	}

	return id, nil
}

// Detect reads CV7 and CV8 to identify the decoder and returns a VolumeChangeable implementation.
func Detect(cv CVAccess) (VolumeChangeable, error) {
	id, err := Identify(cv)
	if err != nil {
		return nil, err
	}

	switch id.Kind {
	case DecoderRailBOX:
		return NewRailboxRB23xx(WithCVAccess(cv)), nil
	case DecoderESU:
		return NewLokSoundv5(cv), nil
	case DecoderZIMO:
		return NewZIMOMS450(cv), nil
	default:
		return nil, fmt.Errorf("unsupported decoder (CV7=%d, CV8=%d)", id.SoftwareVersion, id.ManufacturerID)
	}
}
