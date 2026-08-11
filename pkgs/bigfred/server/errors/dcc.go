package errors

import "errors"

const (
	CodeDCCAddressOutsidePool      = "dcc_address_outside_pool"
	CodeDCCPoolEmpty               = "dcc_pool_empty"
	CodeDCCPoolRangeInvalid        = "dcc_pool_range_invalid"
	CodeDCCPoolOverlap             = "dcc_pool_overlap"
	CodeDCCPoolForbidden           = "forbidden"
	CodeDCCPoolAutoAllocateConflict = "dcc_pool_auto_allocate_conflict"
	CodeNoDCCBusPortsAvailable     = "no_dcc_bus_ports_available"
)

var (
	ErrDCCAddressOutsidePool = errors.New(CodeDCCAddressOutsidePool)
	ErrDCCPoolEmpty          = errors.New(CodeDCCPoolEmpty)
	ErrDCCPoolRangeInvalid   = errors.New(CodeDCCPoolRangeInvalid)
	ErrDCCPoolOverlap        = errors.New(CodeDCCPoolOverlap)
	ErrDCCPoolForbidden      = errors.New(CodeDCCPoolForbidden)
	// ErrDCCPoolAutoAllocateConflict is returned when a create request asks
	// for automatic allocation and also declares explicit ranges.
	ErrDCCPoolAutoAllocateConflict = errors.New(CodeDCCPoolAutoAllocateConflict)
	ErrNoDCCBusPortsAvailable      = errors.New(CodeNoDCCBusPortsAvailable)
)
