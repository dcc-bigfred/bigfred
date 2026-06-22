package decoders

// railboxProductRB2300 is CV #110 for RB 2300 (locomotive sound decoder).
const railboxProductRB2300 = 23

// railboxWagonManufacturerCV8 is the factory CV #8 value on RB 21x0/RB 2110/RB 2112 wagon
// decoders (per RailBOX manual). Locomotive decoders use NMRA manufacturer ID 172 instead.
const railboxWagonManufacturerCV8 = 13

func isRailboxCV8(cv8 int) bool {
	return cv8 == ManufacturerRailBOX || cv8 == railboxWagonManufacturerCV8
}

// railboxIsLocomotive reports whether the decoder is an RB 23xx locomotive sound decoder.
func railboxIsLocomotive(cv CVAccess) (bool, error) {
	cv8, err := cv.ReadCV(8)
	if err != nil {
		return false, err
	}
	if cv8 == railboxWagonManufacturerCV8 {
		return false, nil
	}
	if cv8 != ManufacturerRailBOX {
		return false, nil
	}

	product, err := cv.ReadCV(110)
	if err != nil {
		return false, err
	}
	return product == railboxProductRB2300, nil
}

func railboxDecoderName(cv CVAccess) string {
	cv8, err := cv.ReadCV(8)
	if err == nil && cv8 == railboxWagonManufacturerCV8 {
		return "RailBOX RB 2112"
	}

	locomotive, err := railboxIsLocomotive(cv)
	if err == nil && locomotive {
		return "RailBOX RB23xx"
	}
	return "RailBOX RB 2112"
}
