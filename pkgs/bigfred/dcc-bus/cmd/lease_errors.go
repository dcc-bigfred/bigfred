package cmd

import (
	stderrors "errors"

	buserrors "github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/slotlease"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func leaseErrorResult(userID uint, leaser *slotlease.Leaser, err error) Result {
	code := leaseErrorCode(err)
	res := FailResult(code)
	if leaser != nil && code == buserrors.CodeVehicleCapExceeded {
		res.DrivenAddrs = leaser.DrivenAddrs(userID)
	}
	return res
}

func leaseErrorCode(err error) string {
	switch {
	case stderrors.Is(err, slotlease.ErrNotAllowed):
		return buserrors.CodeNotAllowed
	case stderrors.Is(err, slotlease.ErrVehicleCapExceeded):
		return buserrors.CodeVehicleCapExceeded
	case stderrors.Is(err, commandstation.ErrNoFreeSlot):
		return buserrors.CodeNoFreeSlot
	case stderrors.Is(err, commandstation.ErrSlotInUse):
		return buserrors.CodeSlotInUse
	case stderrors.Is(err, commandstation.ErrBigFredSlotBudgetExceeded):
		return buserrors.CodeBigFredSlotBudgetExceeded
	default:
		return buserrors.CodeCommandStationError
	}
}
