package decoders

import "fmt"

const zimoMappingFnMax = 12

// ZIMOMS450Mapping programs NMRA function-to-output mapping via CV #33–#46.
type ZIMOMS450Mapping struct {
	cv CVAccess
}

func NewZIMOMS450Mapping(cv CVAccess) *ZIMOMS450Mapping {
	return &ZIMOMS450Mapping{cv: cv}
}

func (d *ZIMOMS450Mapping) SetFunctionMapping(fn uint8, outputs []MappingOutput, direction MappingDirection) ([]MappingWrite, error) {
	if d.cv == nil {
		return nil, fmt.Errorf("CV access not configured")
	}
	if fn > zimoMappingFnMax {
		return nil, fmt.Errorf("ZIMO NMRA mapping supports F0-F%d", zimoMappingFnMax)
	}

	value, err := zimoOutputBits(outputs)
	if err != nil {
		return nil, err
	}

	writes := make([]MappingWrite, 0, 2)
	switch direction {
	case MappingForward:
		cv, err := zimoMappingCV(fn, true)
		if err != nil {
			return nil, err
		}
		writes = append(writes, MappingWrite{CV: cv, Value: value})
	case MappingReverse:
		if fn == 0 {
			cv, err := zimoMappingCV(fn, false)
			if err != nil {
				return nil, err
			}
			writes = append(writes, MappingWrite{CV: cv, Value: value})
		} else {
			return nil, fmt.Errorf("only F0 has a separate reverse mapping on ZIMO (CV #34)")
		}
	default:
		if fn == 0 {
			fwdCV, err := zimoMappingCV(fn, true)
			if err != nil {
				return nil, err
			}
			revCV, err := zimoMappingCV(fn, false)
			if err != nil {
				return nil, err
			}
			writes = append(writes,
				MappingWrite{CV: fwdCV, Value: value},
				MappingWrite{CV: revCV, Value: value},
			)
		} else {
			cv, err := zimoMappingCV(fn, true)
			if err != nil {
				return nil, err
			}
			writes = append(writes, MappingWrite{CV: cv, Value: value})
		}
	}

	for _, write := range writes {
		if err := d.cv.WriteCV(write.CV, write.Value); err != nil {
			return nil, fmt.Errorf("failed to write CV%d: %w", write.CV, err)
		}
	}
	return writes, nil
}

func zimoMappingCV(fn uint8, forward bool) (uint16, error) {
	if fn == 0 {
		if forward {
			return 33, nil
		}
		return 34, nil
	}
	if fn > zimoMappingFnMax {
		return 0, fmt.Errorf("function F%d out of range for ZIMO NMRA mapping (F1-F%d)", fn, zimoMappingFnMax)
	}
	return uint16(34 + fn), nil
}

func zimoOutputBits(outputs []MappingOutput) (int, error) {
	var value int
	for _, output := range outputs {
		if output.Kind != OutputNumbered {
			return 0, fmt.Errorf("%s is not available on ZIMO NMRA mapping; use F0 with --forward/--reverse instead", output.Label)
		}
		if output.Number < 1 || output.Number > 8 {
			return 0, fmt.Errorf("output O%d out of range for ZIMO NMRA mapping (FO1-FO8)", output.Number)
		}
		value |= 1 << (output.Number - 1)
	}
	return value, nil
}
