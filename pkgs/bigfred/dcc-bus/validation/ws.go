// Package validation holds stateless input validators for dcc-bus WS payloads.
package validation

import (
	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// LocoSelect validates loco.select payloads.
type LocoSelect struct{}

// Valid reports whether the address is non-zero.
func (LocoSelect) Valid(p protocol.LocoSelectPayload) bool {
	return p.Address != 0
}

// LocoStealSlot validates loco.stealSlot payloads.
type LocoStealSlot struct{}

// Valid reports whether the address is non-zero.
func (LocoStealSlot) Valid(p protocol.LocoStealSlotPayload) bool {
	return p.Address != 0
}

// LocoDeselect validates loco.deselect payloads.
type LocoDeselect struct{}

// Valid reports whether the address is non-zero.
func (LocoDeselect) Valid(p protocol.LocoDeselectPayload) bool {
	return p.Address != 0
}

// TrainSelect validates train.select payloads.
type TrainSelect struct{}

// Valid reports whether the train id is non-empty.
func (TrainSelect) Valid(p protocol.TrainSelectPayload) bool {
	return p.TrainID != ""
}

// LocoSubscribe validates loco.subscribe addresses.
type LocoSubscribe struct{}

// Valid reports whether every address is non-zero.
func (LocoSubscribe) Valid(p protocol.LocoSubscribePayload) bool {
	if len(p.Addresses) == 0 {
		return false
	}
	for _, addr := range p.Addresses {
		if addr == 0 {
			return false
		}
	}
	return true
}

// maxCVNum is the highest CV number addressable by NMRA S-9.2.2
// indexed/paged addressing.
const maxCVNum = 1024

// validProgrammingMode reports whether mode is empty (use the daemon
// default) or one of the two track selectors.
func validProgrammingMode(mode string) bool {
	switch mode {
	case "", protocol.ProgrammingModePOM, protocol.ProgrammingModeProg:
		return true
	default:
		return false
	}
}

// LocoCVWrite validates loco.cvWrite payloads.
type LocoCVWrite struct{}

// Valid reports whether the mode is known and every CV number is in range.
func (LocoCVWrite) Valid(p protocol.LocoCVWritePayload) bool {
	if !validProgrammingMode(p.Mode) || len(p.CVs) == 0 {
		return false
	}
	for _, entry := range p.CVs {
		if entry.CV == 0 || entry.CV > maxCVNum {
			return false
		}
	}
	return true
}

// LocoCVRead validates loco.cvRead payloads.
type LocoCVRead struct{}

// Valid reports whether the mode is known and every CV number is in range.
func (LocoCVRead) Valid(p protocol.LocoCVReadPayload) bool {
	if !validProgrammingMode(p.Mode) || len(p.CVs) == 0 {
		return false
	}
	for _, cv := range p.CVs {
		if cv == 0 || cv > maxCVNum {
			return false
		}
	}
	return true
}

// LocoAddrSet validates loco.addrSet payloads.
type LocoAddrSet struct{}

// Valid reports whether the mode is known and the address is a legal
// DCC address (NMRA S-9.2.2 long-address ceiling).
func (LocoAddrSet) Valid(p protocol.LocoAddrSetPayload) bool {
	return validProgrammingMode(p.Mode) && p.Address >= 1 && p.Address <= 10239
}

// LocoAddrGet validates loco.addrGet payloads.
type LocoAddrGet struct{}

// Valid reports whether the mode is known. The address is optional —
// it is only needed for POM reads, which the router enforces.
func (LocoAddrGet) Valid(p protocol.LocoAddrGetPayload) bool {
	return validProgrammingMode(p.Mode)
}

// SetSpeed validates loco.setSpeed wire payloads against speedSteps.
type SetSpeed struct {
	SpeedSteps uint
}

// Valid reports whether addr and speed are in range.
func (v SetSpeed) Valid(p contract.LocoSetSpeedWire) bool {
	if p.Address == 0 {
		return false
	}
	if v.SpeedSteps == 0 {
		return p.Speed <= 128
	}
	return p.Speed <= uint8(v.SpeedSteps)
}

// SetFunction validates loco.setFunction wire payloads.
type SetFunction struct{}

// Valid reports whether addr and function index are in range.
func (SetFunction) Valid(p contract.LocoSetFunctionWire) bool {
	return p.Address != 0 && p.Function <= 31
}

// TrainSetSpeed validates train.setSpeed wire payloads.
type TrainSetSpeed struct {
	SpeedSteps uint
}

// Valid reports whether train id and speed are in range.
func (v TrainSetSpeed) Valid(p contract.TrainSetSpeedWire) bool {
	if p.TrainID == "" {
		return false
	}
	if v.SpeedSteps == 0 {
		return p.Speed <= 128
	}
	return p.Speed <= uint8(v.SpeedSteps)
}
