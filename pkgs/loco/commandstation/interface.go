package commandstation

import (
	"fmt"
	"time"
)

type LocoCV struct {
	LocoId LocoAddr
	Cv     CV
}

// CV is a par of CVx=y, where y is optional and can be ""
type CV struct {
	Num   CVNum
	Value int
}

func (cv *CV) Repr() string {
	return fmt.Sprintf("%d=%d", cv.Num, cv.Value)
}

func (cv *CV) Translate() uint16 {
	return uint16(cv.Num - 1)
}

type Station interface {
	// WriteCV sends a write request to the command station to write CV of specific value for a given locomotive
	WriteCV(mode Mode, lcv LocoCV, options ...ctxOptions) error
	ReadCV(mode Mode, lcv LocoCV, options ...ctxOptions) (int, error)
	SendFn(mode Mode, addr LocoAddr, num FuncNum, toggle bool) error
	// ListFunctions returns a list of function numbers that are currently active (on) for the given locomotive
	ListFunctions(addr LocoAddr) ([]int, error)
	// SetSpeed sets the speed and direction of a locomotive
	SetSpeed(addr LocoAddr, speed uint8, forward bool, speedSteps uint8) error
	// GetSpeed retrieves the current speed and direction of a locomotive
	GetSpeed(addr LocoAddr) (speed uint8, forward bool, err error)
	CleanUp() error
}

// CV number
type CVNum uint16

// LocoAddr represents locomotive address
type LocoAddr uint16

// Function number
type FuncNum int

// Mode could be PoM or programming track. Depending on what's supported by your command station
type Mode string

const (
	MainTrackMode        Mode = "pom"
	ProgrammingTrackMode Mode = "prog"
)

// internal key for function-group cache
type fnStateKey struct {
	addr   LocoAddr
	fnType byte
}

//
// Contextual options
//

type ctxOptions func(*RequestContext) error

type RequestContext struct {
	timeout time.Duration
	verify  bool
	retries uint8
	settle  time.Duration
}

func Timeout(timeout time.Duration) func(*RequestContext) error {
	return func(ctx *RequestContext) error {
		ctx.timeout = timeout
		return nil
	}
}

func Retries(retries uint8) func(*RequestContext) error {
	return func(ctx *RequestContext) error {
		ctx.retries = retries
		return nil
	}
}

func Verify(shouldVerify bool) func(*RequestContext) error {
	return func(ctx *RequestContext) error {
		ctx.verify = shouldVerify
		return nil
	}
}

func applyMethodsToCtx(ctx *RequestContext, options []ctxOptions) {
	for _, option := range options {
		option(ctx)
	}
}

// --- End of contextual options ---
