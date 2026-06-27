package errors

import "errors"

const CodeWithrottleServerDisabled = "withrottle_server_disabled"

var ErrWithrottleServerDisabled = errors.New(CodeWithrottleServerDisabled)
