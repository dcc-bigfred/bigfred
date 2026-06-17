package errors

import "errors"

const (
	CodeSudoInvalidInput   = "sudo_invalid_input"
	CodeSudoInvalidPIN     = "sudo_invalid_pin"
	CodeSudoLocked         = "sudo_locked"
	CodeSudoLayoutMismatch = "sudo_layout_mismatch"
)

var (
	ErrSudoInvalidInput = errors.New(CodeSudoInvalidInput)
	ErrSudoInvalidPIN   = errors.New(CodeSudoInvalidPIN)
	ErrSudoLocked       = errors.New(CodeSudoLocked)
)
