package cmd

import (
	stderrors "errors"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func locoCommandErrorCode(err error) string {
	switch {
	case stderrors.Is(err, commandstation.ErrSlotBusUnavailable):
		return errors.CodeSlotBusUnavailable
	case stderrors.Is(err, commandstation.ErrNoFreeSlot):
		return errors.CodeNoFreeSlot
	case stderrors.Is(err, commandstation.ErrSlotInUse):
		return errors.CodeSlotInUse
	case stderrors.Is(err, commandstation.ErrBigFredSlotBudgetExceeded):
		return errors.CodeBigFredSlotBudgetExceeded
	default:
		return errors.CodeCommandStationError
	}
}
