package cmd

import (
	"fmt"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// Address encoding per NMRA S-9.2.2. Mirrors pkgs/loco/app/addr.go,
// duplicated here because those helpers are unexported and tied to the
// CLI's LocoApp lifecycle (InitializeCommandStation / CleanUp), which
// the long-lived daemon must not run per request.
const (
	cv29LongAddressBit = 32 // bit 5: address is taken from CV17/CV18

	shortAddressMin = 1
	shortAddressMax = 127
	longAddressMax  = 10239
)

// addressCVNums are the CVs that together encode a decoder address.
var addressCVNums = []uint16{1, 17, 18, 29}

// addressFromCVs decodes a decoder address out of its four address CVs.
func addressFromCVs(cv1, cv17, cv18, cv29 int) (addr uint16, long bool, err error) {
	if cv29&cv29LongAddressBit != 0 {
		if cv17 < 192 {
			return 0, false, fmt.Errorf("invalid long address: CV17=%d (expected >= 192)", cv17)
		}
		return uint16((cv17-192)*256 + cv18), true, nil
	}
	if cv1 < shortAddressMin || cv1 > shortAddressMax {
		return 0, false, fmt.Errorf("invalid short address: CV1=%d (expected %d-%d)", cv1, shortAddressMin, shortAddressMax)
	}
	return uint16(cv1), false, nil
}

// addressCVWrites builds the CV writes that program addr, preserving
// every CV29 bit other than the long-address bit.
func addressCVWrites(addr uint16, cv29 int) ([]protocol.CVEntry, bool, error) {
	if addr > longAddressMax {
		return nil, false, fmt.Errorf("address %d out of range (0-%d)", addr, longAddressMax)
	}
	if addr >= shortAddressMin && addr <= shortAddressMax {
		return []protocol.CVEntry{
			{CV: 1, Value: uint8(addr)},
			{CV: 17, Value: 0},
			{CV: 18, Value: 0},
			{CV: 29, Value: uint8(cv29 &^ cv29LongAddressBit)},
		}, false, nil
	}
	return []protocol.CVEntry{
		{CV: 17, Value: uint8(192 + addr/256)},
		{CV: 18, Value: uint8(addr % 256)},
		{CV: 29, Value: uint8(cv29 | cv29LongAddressBit)},
	}, true, nil
}
