package supervisord

import "errors"

var (
	ErrInvalidProgramName = errors.New("supervisord: invalid program name")
	ErrInvalidCommand     = errors.New("supervisord: command is required")
	ErrDuplicateProgram   = errors.New("supervisord: duplicate program name")
	ErrProgramInMultiGroup = errors.New("supervisord: program belongs to multiple groups")
	ErrGroupNotFound      = errors.New("supervisord: group not found")
	ErrProgramNotFound    = errors.New("supervisord: program not found")
	ErrBinaryNotFound     = errors.New("supervisord: binary not found on PATH")
	ErrDaemonNotRunning   = errors.New("supervisord: daemon not running")
	ErrReloadFailed       = errors.New("supervisord: reload failed")
	ErrDaemonRestart      = errors.New("supervisord: daemon restart failed")
)
