package errors

import "errors"

const (
	CodeInvalidCredentials = "invalid_credentials"
	CodeAccountDeactivated = "account_deactivated"
)

var (
	// ErrInvalidCredentials intentionally covers unknown login and wrong PIN.
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDeactivated = errors.New(CodeAccountDeactivated)
)
