package errors

import "errors"

const (
	CodeLeaseNotFound           = "lease_not_found"
	CodeLeaseConflict           = "lease_conflict"
	CodeLeaseNotOwner           = "lease_not_owner"
	CodeLeaseNotParty           = "lease_not_party"
	CodeLeaseSelf               = "lease_self"
	CodeLeaseTargetNotOnLayout  = "lease_target_not_on_layout"
	CodeLeaseInvalidSpeedLimit  = "lease_invalid_speed_limit"
	CodeLeaseInvalidDuration    = "lease_invalid_duration"
	CodeLeaseTargetNotDrivable  = "lease_target_not_drivable"
	CodeLeaseStoreUnavailable = "lease_store_unavailable"
)

var (
	ErrLeaseNotFound          = errors.New(CodeLeaseNotFound)
	ErrLeaseStoreUnavailable  = errors.New(CodeLeaseStoreUnavailable)
	ErrLeaseConflict          = errors.New(CodeLeaseConflict)
	ErrLeaseNotOwner          = errors.New(CodeLeaseNotOwner)
	ErrLeaseNotParty          = errors.New(CodeLeaseNotParty)
	ErrLeaseSelf              = errors.New(CodeLeaseSelf)
	ErrLeaseTargetNotOnLayout = errors.New(CodeLeaseTargetNotOnLayout)
	ErrLeaseInvalidSpeedLimit = errors.New(CodeLeaseInvalidSpeedLimit)
	ErrLeaseInvalidDuration   = errors.New(CodeLeaseInvalidDuration)
	ErrLeaseTargetNotDrivable = errors.New(CodeLeaseTargetNotDrivable)
)
