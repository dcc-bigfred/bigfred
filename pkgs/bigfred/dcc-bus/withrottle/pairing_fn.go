package withrottle

import (
	"strconv"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

const pairingFnBufferSize = 6

// PairingDigitsFromFn maps a pressed function key to one or two decimal digits
// entered while pairing: F0–F9 → one digit, F10–F32 → two digits.
func PairingDigitsFromFn(fn int) (string, bool) {
	if fn < 0 || fn > 32 {
		return "", false
	}
	return strconv.Itoa(fn), true
}

func pairingCodeFromFragments(fragments []string) (code string, ok bool) {
	concat := strings.Join(fragments, "")
	if !contract.ValidWithrottleCode(concat) {
		return "", false
	}
	return concat, true
}

func (c *wireClient) BufferPairingFn(fn int) (code string, ready bool) {
	frag, ok := PairingDigitsFromFn(fn)
	if !ok {
		return "", false
	}
	c.pairFnBuf = append(c.pairFnBuf, frag)
	if len(c.pairFnBuf) > pairingFnBufferSize {
		c.pairFnBuf = c.pairFnBuf[len(c.pairFnBuf)-pairingFnBufferSize:]
	}
	return pairingCodeFromFragments(c.pairFnBuf)
}

func (c *wireClient) pairingFnRisingEdge(fn int, on bool) bool {
	if c.pairFnPrevFn == nil {
		c.pairFnPrevFn = make(map[int]bool)
	}
	prev := c.pairFnPrevFn[fn]
	c.pairFnPrevFn[fn] = on
	return on && !prev
}

func (c *wireClient) clearPairingBuffer() {
	c.pairFnBuf = nil
	c.pairFnPrevFn = nil
}
