package errors

import "errors"

const (
	CodeInterlockingNotFound     = "interlocking_not_found"
	CodeInterlockingNameTaken    = "interlocking_name_taken"
	CodeInterlockingNameRequired = "interlocking_name_required"
	CodeInterlockingForbidden    = "forbidden"
	CodeInterlockingOccupied     = "interlocking_occupied"
	CodeInterlockingNotInLayout  = "interlocking_not_in_layout"
	CodeNotSignalman             = "not_signalman"
)

var (
	ErrInterlockingNotFound     = errors.New(CodeInterlockingNotFound)
	ErrInterlockingNameTaken    = errors.New(CodeInterlockingNameTaken)
	ErrInterlockingNameRequired = errors.New(CodeInterlockingNameRequired)
	ErrInterlockingForbidden    = errors.New(CodeInterlockingForbidden)
	ErrInterlockingOccupied     = errors.New(CodeInterlockingOccupied)
	ErrInterlockingNotInLayout  = errors.New(CodeInterlockingNotInLayout)
	ErrNotSignalman             = errors.New(CodeNotSignalman)
)
