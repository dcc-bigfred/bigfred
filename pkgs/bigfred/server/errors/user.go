package errors

import "errors"

const (
	CodeUserNotFound         = "user_not_found"
	CodeUserLoginRequired    = "user_login_required"
	CodeUserLoginInvalid     = "user_login_invalid"
	CodeUserLoginTaken       = "user_login_taken"
	CodeUserPINRequired      = "user_pin_required"
	CodeUserPINInvalid       = "user_pin_invalid"
	CodeUserRoleInvalid      = "user_role_invalid"
	CodeUserHasVehicles      = "user_has_vehicles"
	CodeUserHasTrains        = "user_has_trains"
	CodeCannotDeactivateSelf = "cannot_deactivate_self"
	CodeCannotDeleteSelf     = "cannot_delete_self"
	CodeUserForbidden        = "forbidden"
)

var (
	ErrUserNotFound         = errors.New(CodeUserNotFound)
	ErrUserLoginRequired    = errors.New(CodeUserLoginRequired)
	ErrUserLoginInvalid     = errors.New(CodeUserLoginInvalid)
	ErrUserLoginTaken       = errors.New(CodeUserLoginTaken)
	ErrUserPINRequired      = errors.New(CodeUserPINRequired)
	ErrUserPINInvalid       = errors.New(CodeUserPINInvalid)
	ErrUserRoleInvalid      = errors.New(CodeUserRoleInvalid)
	ErrUserHasVehicles      = errors.New(CodeUserHasVehicles)
	ErrUserHasTrains        = errors.New(CodeUserHasTrains)
	ErrCannotDeactivateSelf = errors.New(CodeCannotDeactivateSelf)
	ErrCannotDeleteSelf     = errors.New(CodeCannotDeleteSelf)
	ErrUserForbidden        = errors.New(CodeUserForbidden)
)
