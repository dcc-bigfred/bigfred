package errors

import "errors"

const (
	CodeRemoteProtocolUnknown    = "remote_protocol_unknown"
	CodeRemoteUserAlreadyPaired  = "remote_user_already_paired"
)

var (
	ErrRemoteProtocolUnknown   = errors.New(CodeRemoteProtocolUnknown)
	ErrRemoteUserAlreadyPaired = errors.New(CodeRemoteUserAlreadyPaired)
)
