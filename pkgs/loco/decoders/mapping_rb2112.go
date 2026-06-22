package decoders

import "fmt"

const (
	railboxRB2112MappingCVBase1 = 120
	railboxRB2112MappingCVBase2 = 190
	railboxRB2112OutputMax      = 14
)

// RailboxRB2112Mapping programs function-to-output mapping via CV #120–#177 and #190–#247.
//
// Bank 1 (CV #120+) bit layout: bit0=F0_F, bit1=F0_R, bit2=O2 … bit7=O7.
// Bank 2 (CV #190+) bit layout: bit0=O8, bit1=O9 … bit6=O14.
type RailboxRB2112Mapping struct {
	cv CVAccess
}

func NewRailboxRB2112Mapping(cv CVAccess) *RailboxRB2112Mapping {
	return &RailboxRB2112Mapping{cv: cv}
}

func (d *RailboxRB2112Mapping) SetFunctionMapping(fn uint8, outputs []MappingOutput, direction MappingDirection) ([]MappingWrite, error) {
	if d.cv == nil {
		return nil, fmt.Errorf("CV access not configured")
	}
	if fn > 28 {
		return nil, fmt.Errorf("function F%d out of range (F0-F28)", fn)
	}

	bank1, bank2, err := rb2112SplitOutputs(outputs)
	if err != nil {
		return nil, err
	}

	writes := make([]MappingWrite, 0, 4)
	appendWrites := func(forward bool) {
		if bank1 >= 0 {
			writes = append(writes, MappingWrite{CV: rb2112MappingCV(fn, forward, 1), Value: bank1})
		}
		if bank2 >= 0 {
			writes = append(writes, MappingWrite{CV: rb2112MappingCV(fn, forward, 2), Value: bank2})
		}
	}

	switch direction {
	case MappingForward:
		appendWrites(true)
	case MappingReverse:
		appendWrites(false)
	default:
		appendWrites(true)
		appendWrites(false)
	}

	if len(writes) == 0 {
		return nil, fmt.Errorf("no mapping CVs produced for F%d", fn)
	}

	for _, write := range writes {
		if err := d.cv.WriteCV(write.CV, write.Value); err != nil {
			return nil, fmt.Errorf("failed to write CV%d: %w", write.CV, err)
		}
	}
	return writes, nil
}

func rb2112MappingCV(fn uint8, forward bool, bank int) uint16 {
	offset := 0
	if !forward {
		offset = 1
	}
	base := railboxRB2112MappingCVBase1
	if bank == 2 {
		base = railboxRB2112MappingCVBase2
	}
	return uint16(base + int(fn)*2 + offset)
}

// rb2112SplitOutputs maps requested outputs to bank1/bank2 bit masks.
// A bank value of -1 means that bank is not written.
func rb2112SplitOutputs(outputs []MappingOutput) (bank1 int, bank2 int, err error) {
	bank1 = -1
	bank2 = -1

	setBank1 := func(bit uint) {
		if bank1 < 0 {
			bank1 = 0
		}
		bank1 |= 1 << bit
	}

	for _, output := range outputs {
		switch output.Kind {
		case OutputF0Forward:
			setBank1(0)
		case OutputF0Reverse:
			setBank1(1)
		case OutputNumbered:
			n := output.Number
			if n < 2 || n > railboxRB2112OutputMax {
				return 0, 0, fmt.Errorf("output O%d out of range (use O2-O%d, or F0_F/F0_R for output 1)", n, railboxRB2112OutputMax)
			}
			if n <= 7 {
				setBank1(uint(n))
			} else {
				if bank2 < 0 {
					bank2 = 0
				}
				bank2 |= 1 << (n - 8)
			}
		}
	}
	return bank1, bank2, nil
}
