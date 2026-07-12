package errors

import "errors"

const (
	CodeRadioNotConfigured      = "radio_not_configured"
	CodeRadioContextUnavailable = "radio_context_unavailable"
	CodeRadioChatDisabled       = "radio_chat_disabled"
)

var (
	ErrRadioNotConfigured      = errors.New(CodeRadioNotConfigured)
	ErrRadioContextUnavailable = errors.New(CodeRadioContextUnavailable)
	ErrRadioChatDisabled       = errors.New(CodeRadioChatDisabled)
)
