package errors

import "errors"

const (
	CodeRadioNotConfigured      = "radio_not_configured"
	CodeRadioContextUnavailable = "radio_context_unavailable"
)

var (
	ErrRadioNotConfigured      = errors.New(CodeRadioNotConfigured)
	ErrRadioContextUnavailable = errors.New(CodeRadioContextUnavailable)
)
