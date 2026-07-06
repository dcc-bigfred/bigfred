package errors

import "errors"

const (
	CodeLayoutNotFound            = "layout_not_found"
	CodeLayoutLocked              = "layout_locked"
	CodeLayoutNameTaken           = "layout_name_taken"
	CodeLayoutNameRequired        = "layout_name_required"
	CodeSystemLayoutImmutable     = "default_layout_immutable"
	CodeSystemLayoutUndeletable   = "default_layout_undeletable"
	CodeLayoutAdminPINInvalid     = "layout_admin_pin_invalid"
	CodeLayoutAdminPINUnset       = "layout_admin_pin_unset"
	CodeLayoutAdminPINMismatch    = "layout_admin_pin_mismatch"
	CodeLayoutForbidden           = "forbidden"
	CodeLayoutMaxVehiclesInvalid  = "layout_max_vehicles_invalid"
	CodeLayoutMaxVehiclesExceedsSlotBudget = "layout_max_vehicles_exceeds_slot_budget"
)

var (
	ErrLayoutNotFound            = errors.New(CodeLayoutNotFound)
	ErrLayoutLocked              = errors.New(CodeLayoutLocked)
	ErrLayoutNameTaken           = errors.New(CodeLayoutNameTaken)
	ErrLayoutNameRequired        = errors.New(CodeLayoutNameRequired)
	ErrSystemLayoutImmutable     = errors.New(CodeSystemLayoutImmutable)
	ErrSystemLayoutUndeletable   = errors.New(CodeSystemLayoutUndeletable)
	ErrLayoutAdminPINInvalid     = errors.New(CodeLayoutAdminPINInvalid)
	ErrLayoutAdminPINUnset       = errors.New(CodeLayoutAdminPINUnset)
	ErrLayoutAdminPINMismatch    = errors.New(CodeLayoutAdminPINMismatch)
	ErrLayoutForbidden           = errors.New(CodeLayoutForbidden)
	ErrLayoutMaxVehiclesInvalid  = errors.New(CodeLayoutMaxVehiclesInvalid)
	ErrLayoutMaxVehiclesExceedsSlotBudget = errors.New(CodeLayoutMaxVehiclesExceedsSlotBudget)
)
