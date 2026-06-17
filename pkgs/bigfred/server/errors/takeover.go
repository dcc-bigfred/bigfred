package errors

import "errors"

const (
	CodeTakeoverNotConfigured     = "takeover_not_configured"
	CodeTakeoverTargetNotOnLayout = "takeover_target_not_on_layout"
	CodeTakeoverNotOwner          = "takeover_not_owner"
	CodeTakeoverAlreadyPending    = "takeover_already_pending"
	CodeTakeoverNotFound          = "takeover_not_found"
	CodeTakeoverInvalidState      = "takeover_invalid_state"
	CodeTakeoverNotDriver         = "not_takeover_driver"
	CodeTakeoverNotSignalman      = "not_takeover_signalman"
	CodeNotInterlockingOccupant   = "not_interlocking_occupant"
)

var (
	ErrTakeoverNotConfigured     = errors.New(CodeTakeoverNotConfigured)
	ErrTakeoverTargetNotOnLayout = errors.New(CodeTakeoverTargetNotOnLayout)
	ErrTakeoverNotOwner          = errors.New(CodeTakeoverNotOwner)
	ErrTakeoverAlreadyPending    = errors.New(CodeTakeoverAlreadyPending)
	ErrTakeoverNotFound          = errors.New(CodeTakeoverNotFound)
	ErrTakeoverInvalidState      = errors.New(CodeTakeoverInvalidState)
	ErrTakeoverNotDriver         = errors.New(CodeTakeoverNotDriver)
	ErrTakeoverNotSignalman      = errors.New(CodeTakeoverNotSignalman)
	ErrNotInterlockingOccupant   = errors.New(CodeNotInterlockingOccupant)
)
