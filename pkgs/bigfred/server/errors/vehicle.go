package errors

import "errors"

// Vehicle catalogue error codes (REST + service sentinels).
const (
	CodeVehicleNotFound            = "vehicle_not_found"
	CodeVehicleNameRequired        = "vehicle_name_required"
	CodeVehicleKindInvalid         = "vehicle_kind_invalid"
	CodeDCCAddressTaken            = "dcc_address_taken"
	CodeVehicleNotOwned            = "vehicle_not_owned"
	CodeVehicleInUse               = "vehicle_in_use"
	CodeVehicleDccFunctionInvalid   = "vehicle_dcc_function_invalid"
	CodeVehicleDeadManSwitchInvalid = "vehicle_deadman_switch_invalid"
	CodeVehicleEpochInvalid         = "vehicle_epoch_invalid"
	CodeVehicleRevisionDateInvalid  = "vehicle_revision_date_invalid"
)

var (
	ErrVehicleNotFound             = errors.New(CodeVehicleNotFound)
	ErrVehicleNameRequired         = errors.New(CodeVehicleNameRequired)
	ErrVehicleKindInvalid          = errors.New(CodeVehicleKindInvalid)
	ErrDCCAddressTaken             = errors.New(CodeDCCAddressTaken)
	ErrVehicleNotOwned             = errors.New(CodeVehicleNotOwned)
	ErrVehicleInUse                = errors.New(CodeVehicleInUse)
	ErrVehicleDccFunctionInvalid   = errors.New(CodeVehicleDccFunctionInvalid)
	ErrVehicleDeadManSwitchInvalid = errors.New(CodeVehicleDeadManSwitchInvalid)
	ErrVehicleEpochInvalid         = errors.New(CodeVehicleEpochInvalid)
	ErrVehicleRevisionDateInvalid  = errors.New(CodeVehicleRevisionDateInvalid)
)
