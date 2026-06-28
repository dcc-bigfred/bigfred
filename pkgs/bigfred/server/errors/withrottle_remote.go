package errors

import "errors"

const (
	CodeWithrottleServerDisabled = "withrottle_server_disabled"
	CodeWithrottleSessionNotFound = "withrottle_session_not_found"
)

var (
	ErrWithrottleServerDisabled  = errors.New(CodeWithrottleServerDisabled)
	ErrWithrottleSessionNotFound = errors.New(CodeWithrottleSessionNotFound)
)
