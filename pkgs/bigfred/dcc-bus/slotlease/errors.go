package slotlease

import (
	"errors"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

var (
	// ErrNoFreeSlot is returned when the command station has no free slot.
	ErrNoFreeSlot = commandstation.ErrNoFreeSlot

	// ErrBigFredSlotBudgetExceeded is returned when max_loconet_slots is exhausted.
	ErrBigFredSlotBudgetExceeded = commandstation.ErrBigFredSlotBudgetExceeded

	// ErrVehicleCapExceeded is returned when the per-user driven-vehicle cap
	// cannot accommodate a new lease without explicit deselection.
	ErrVehicleCapExceeded = errors.New("slotlease: vehicle cap exceeded")

	// ErrNotAllowed is returned when the drive gate denies the request.
	ErrNotAllowed = errors.New("slotlease: not allowed to drive")
)
