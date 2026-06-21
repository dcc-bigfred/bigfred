package decoders

import "fmt"

// FactoryResetCVValue returns the CV8 write value that triggers a factory reset.
func FactoryResetCVValue(kind DecoderKind) (int, error) {
	switch kind {
	case DecoderRailBOX:
		return 1, nil
	case DecoderESU, DecoderZIMO:
		return 8, nil
	default:
		return 0, fmt.Errorf("factory reset is not supported for decoder kind %v", kind)
	}
}
