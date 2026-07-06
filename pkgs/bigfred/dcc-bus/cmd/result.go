package cmd

import "github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"

// Result is the outcome of a use-case action before the WS layer maps it
// onto ack / loco.error frames.
type Result struct {
	OK          bool
	Code        string
	Members     []protocol.TrainSetSpeedMemberAck
	EvictedAddr uint16
	DrivenAddrs []uint16
}

// OKResult returns a successful action result.
func OKResult() Result {
	return Result{OK: true}
}

// FailResult returns a failed action result with a machine-readable code.
func FailResult(code string) Result {
	return Result{OK: false, Code: code}
}
