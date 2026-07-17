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
