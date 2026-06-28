package errors

import "errors"

const (
	CodeRemoteProtocolUnknown    = "remote_protocol_unknown"
	CodeRemoteUserAlreadyPaired  = "remote_user_already_paired"
	CodeRemoteServerDisabled     = "remote_server_disabled"
	CodeRemoteSessionNotFound    = "remote_session_not_found"
)

var (
	ErrRemoteProtocolUnknown   = errors.New(CodeRemoteProtocolUnknown)
	ErrRemoteUserAlreadyPaired = errors.New(CodeRemoteUserAlreadyPaired)
	ErrRemoteServerDisabled    = errors.New(CodeRemoteServerDisabled)
	ErrRemoteSessionNotFound   = errors.New(CodeRemoteSessionNotFound)
)
