package errors

import "errors"

const (
	CodeDCCAddressOutsidePool  = "dcc_address_outside_pool"
	CodeDCCPoolEmpty           = "dcc_pool_empty"
	CodeDCCPoolRangeInvalid    = "dcc_pool_range_invalid"
	CodeDCCPoolOverlap         = "dcc_pool_overlap"
	CodeDCCPoolForbidden       = "forbidden"
	CodeNoDCCBusPortsAvailable = "no_dcc_bus_ports_available"
)

var (
	ErrDCCAddressOutsidePool  = errors.New(CodeDCCAddressOutsidePool)
	ErrDCCPoolEmpty           = errors.New(CodeDCCPoolEmpty)
	ErrDCCPoolRangeInvalid    = errors.New(CodeDCCPoolRangeInvalid)
	ErrDCCPoolOverlap         = errors.New(CodeDCCPoolOverlap)
	ErrDCCPoolForbidden       = errors.New(CodeDCCPoolForbidden)
	ErrNoDCCBusPortsAvailable = errors.New(CodeNoDCCBusPortsAvailable)
)
