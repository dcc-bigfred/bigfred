package z21server

import (
	"strconv"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

const pairingFnBufferSize = 6

// pairingDigitsFromFn maps a pressed function key to one or two decimal digits
// entered while pairing: F0–F9 → one digit, F10–F32 → two digits.
func pairingDigitsFromFn(fn int) (string, bool) {
	if fn < 0 || fn > 32 {
		return "", false
	}
	return strconv.Itoa(fn), true
}

// parsePairingCodeDigits splits a 6-digit string into CV3 and CV4 values.
func parsePairingCodeDigits(digits string) (cv3, cv4 int, ok bool) {
	if len(digits) != 6 {
		return 0, 0, false
	}
	cv3, err := strconv.Atoi(digits[:3])
	if err != nil {
		return 0, 0, false
	}
	cv4, err = strconv.Atoi(digits[3:])
	if err != nil {
		return 0, 0, false
	}
	if !contract.ValidPairingCV(cv3) || !contract.ValidPairingCV(cv4) {
		return 0, 0, false
	}
	return cv3, cv4, true
}

// pairingCodeFromFragments returns CV3/CV4 when the concatenation of digit
// fragments is exactly six decimal digits forming a valid pairing code.
func pairingCodeFromFragments(fragments []string) (cv3, cv4 int, ok bool) {
	concat := strings.Join(fragments, "")
	if len(concat) != 6 {
		return 0, 0, false
	}
	return parsePairingCodeDigits(concat)
}

func (c *wireClient) bufferPairingFn(fn int) (cv3, cv4 int, ready bool) {
	frag, ok := pairingDigitsFromFn(fn)
	if !ok {
		return 0, 0, false
	}
	c.pairFnBuf = append(c.pairFnBuf, frag)
	if len(c.pairFnBuf) > pairingFnBufferSize {
		c.pairFnBuf = c.pairFnBuf[len(c.pairFnBuf)-pairingFnBufferSize:]
	}
	return pairingCodeFromFragments(c.pairFnBuf)
}

func (c *wireClient) pairingFnRisingEdges(group, fnByte byte) []int {
	fnMap, ok := locoFunctionGroupMap[group]
	if !ok {
		return nil
	}
	if c.pairFnPrevGroup == nil {
		c.pairFnPrevGroup = make(map[byte]byte)
	}
	prev := c.pairFnPrevGroup[group]
	c.pairFnPrevGroup[group] = fnByte
	var risen []int
	for bit, fn := range fnMap {
		if fn < 0 {
			continue
		}
		mask := byte(1 << uint(bit))
		if fnByte&mask != 0 && prev&mask == 0 {
			risen = append(risen, fn)
		}
	}
	return risen
}

func (c *wireClient) clearPairingBuffer() {
	c.pairCV3 = nil
	c.pairCV4 = nil
	c.pairFnBuf = nil
	c.pairFnPrevGroup = nil
}

func (c *wireClient) bufferPairingCV(cvWire int, value int) (cv3, cv4 int, ready bool) {
	switch cvWire {
	case 2:
		c.pairCV3 = &value
	case 3:
		c.pairCV4 = &value
	default:
		return 0, 0, false
	}
	if c.pairCV3 != nil && c.pairCV4 != nil {
		return *c.pairCV3, *c.pairCV4, true
	}
	return 0, 0, false
}
