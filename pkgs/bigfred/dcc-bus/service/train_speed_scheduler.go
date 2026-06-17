package service

import (
	"context"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

// StartDelayMaxPreviousSpeed is the highest DCC step at which a consist
// move still counts as a "start" for trailing start-delay scheduling.
const StartDelayMaxPreviousSpeed uint8 = 1

// MinSpeedRampStepMs is the minimum sleep interval between ramp steps.
const MinSpeedRampStepMs = 500

// MinAccelRampStepMs is deprecated alias for MinSpeedRampStepMs.
const MinAccelRampStepMs = MinSpeedRampStepMs

// TrainMemberSpeedApply writes one member speed command to DCC + Redis.
type TrainMemberSpeedApply func(ctx context.Context, addr uint16, speed uint8, forward bool) error

// TrainMemberSetSpeed is one powered consist member target.
type TrainMemberSetSpeed struct {
	Addr              uint16
	CurrentSpeed      uint8
	Speed             uint8
	Forward           bool
	StartDelayMs      int
	AccelRampMs       int
	AccelRampMaxSteps int
	BrakeRampMs       int
	BrakeRampMaxSteps int
}

type startDelayJob struct {
	addr    uint16
	target  uint8
	forward bool
	delayMs int
}

type speedRampJob struct {
	addr     uint16
	current  uint8
	target   uint8
	forward  bool
	rampMs   int
	maxSteps int
}

// TrainSpeedScheduler fans train.setSpeed to members, optionally delaying
// trailing starts or ramping acceleration in cancellable background goroutines.
type TrainSpeedScheduler struct {
	mu      sync.Mutex
	pending map[uint]context.CancelFunc
}

// NewTrainSpeedScheduler returns a scheduler with an empty delay registry.
func NewTrainSpeedScheduler() *TrainSpeedScheduler {
	return &TrainSpeedScheduler{
		pending: make(map[uint]context.CancelFunc),
	}
}

// CancelTrain aborts any pending delay/ramp goroutines for the train.
func (s *TrainSpeedScheduler) CancelTrain(trainID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.pending[trainID]; ok {
		cancel()
		delete(s.pending, trainID)
	}
}

// IsStartDelayPreviousSpeed reports whether a stored speed still counts
// as a consist "start" for trailing start-delay scheduling.
func IsStartDelayPreviousSpeed(speed uint8) bool {
	return speed <= StartDelayMaxPreviousSpeed
}

// ShouldApplyAccelRamp reports whether a member should ramp while
// accelerating. Accel ramp takes precedence over start delay when both are set.
// It applies when current speed is above the start threshold, or when start
// delay is disabled.
func ShouldApplyAccelRamp(
	commandSpeed uint8,
	currentSpeed uint8,
	targetSpeed uint8,
	startDelayMs int,
	accelRampMs int,
) bool {
	if commandSpeed == 0 || accelRampMs <= 0 || targetSpeed <= currentSpeed {
		return false
	}
	return currentSpeed > StartDelayMaxPreviousSpeed || startDelayMs == 0
}

// ShouldApplyBrakeRamp reports whether a member should ramp while decelerating.
func ShouldApplyBrakeRamp(currentSpeed, targetSpeed uint8, brakeRampMs int) bool {
	return brakeRampMs > 0 && targetSpeed < currentSpeed
}

// ShouldApplyStartDelay reports whether a member should wait before
// the first speed command on a consist start from standstill. Suppressed when
// accel ramp would apply instead.
func ShouldApplyStartDelay(
	commandSpeed uint8,
	leadingWasAtStartSpeed bool,
	currentSpeed uint8,
	targetSpeed uint8,
	startDelayMs int,
	accelRampMs int,
) bool {
	if commandSpeed == 0 || !leadingWasAtStartSpeed || startDelayMs <= 0 {
		return false
	}
	if currentSpeed > StartDelayMaxPreviousSpeed || targetSpeed <= currentSpeed {
		return false
	}
	return !ShouldApplyAccelRamp(
		commandSpeed, currentSpeed, targetSpeed, startDelayMs, accelRampMs,
	)
}

// PlanAccelRamp computes step count and interval for an acceleration ramp.
func PlanAccelRamp(current, target uint8, rampMs, maxSteps int) (steps int, intervalMs int, ok bool) {
	if rampMs <= 0 || maxSteps <= 0 || target <= current {
		return 0, 0, false
	}
	return planSpeedRampIntervals(rampMs, maxSteps)
}

// PlanBrakeRamp computes step count and interval for a braking ramp.
func PlanBrakeRamp(current, target uint8, rampMs, maxSteps int) (steps int, intervalMs int, ok bool) {
	if rampMs <= 0 || maxSteps <= 0 || target >= current {
		return 0, 0, false
	}
	return planSpeedRampIntervals(rampMs, maxSteps)
}

func planSpeedRampIntervals(rampMs, maxSteps int) (steps int, intervalMs int, ok bool) {
	steps = maxSteps
	for steps > 1 && rampMs/steps < MinSpeedRampStepMs {
		steps--
	}
	if steps < 1 {
		steps = 1
	}
	return steps, rampMs / steps, true
}

// PlanAccelRampStepDelta computes the uniform per-step speed increment and
// direction between current and target over the given number of ramp steps.
func PlanAccelRampStepDelta(current, target uint8, steps int) (perStep int, sign int) {
	if steps < 1 {
		return 0, 1
	}
	diff := int(target) - int(current)
	absDiff := diff
	if absDiff < 0 {
		absDiff = -absDiff
	}
	perStep = absDiff / steps
	sign = 1
	if diff < 0 {
		sign = -1
	}
	return perStep, sign
}

// AccelRampSpeeds lists each DCC speed command in a ramp, ending at target.
func AccelRampSpeeds(current, target uint8, steps int) []uint8 {
	if steps < 1 {
		return []uint8{target}
	}
	perStep, sign := PlanAccelRampStepDelta(current, target, steps)
	speeds := make([]uint8, steps)
	speed := int(current)
	for i := 0; i < steps; i++ {
		if i < steps-1 {
			speed += sign * perStep
		} else {
			speed = int(target)
		}
		speeds[i] = uint8(speed)
	}
	return speeds
}

// Apply fans speed to every member in cmds. Immediate members are written
// synchronously; delayed/ramping members run in one goroutine each,
// cancellable by the next Apply/CancelTrain for the same trainID.
func (s *TrainSpeedScheduler) Apply(
	ctx context.Context,
	trainID uint,
	commandSpeed uint8,
	leadingWasAtStartSpeed bool,
	apply TrainMemberSpeedApply,
	members []TrainMemberSetSpeed,
) ([]protocol.TrainSetSpeedMemberAck, bool) {
	s.CancelTrain(trainID)

	acks := make([]protocol.TrainSetSpeedMemberAck, 0, len(members))
	startDelays := make([]startDelayJob, 0, len(members))
	ramps := make([]speedRampJob, 0, len(members))
	allOK := true

	for _, m := range members {
		if ShouldApplyAccelRamp(
			commandSpeed, m.CurrentSpeed, m.Speed, m.StartDelayMs, m.AccelRampMs,
		) {
			if _, _, canRamp := PlanAccelRamp(m.CurrentSpeed, m.Speed, m.AccelRampMs, m.AccelRampMaxSteps); canRamp {
				ramps = append(ramps, speedRampJob{
					addr:     m.Addr,
					current:  m.CurrentSpeed,
					target:   m.Speed,
					forward:  m.Forward,
					rampMs:   m.AccelRampMs,
					maxSteps: m.AccelRampMaxSteps,
				})
				acks = append(acks, protocol.TrainSetSpeedMemberAck{Addr: m.Addr, OK: true})
				continue
			}
		}

		if ShouldApplyBrakeRamp(m.CurrentSpeed, m.Speed, m.BrakeRampMs) {
			if _, _, canRamp := PlanBrakeRamp(m.CurrentSpeed, m.Speed, m.BrakeRampMs, m.BrakeRampMaxSteps); canRamp {
				ramps = append(ramps, speedRampJob{
					addr:     m.Addr,
					current:  m.CurrentSpeed,
					target:   m.Speed,
					forward:  m.Forward,
					rampMs:   m.BrakeRampMs,
					maxSteps: m.BrakeRampMaxSteps,
				})
				acks = append(acks, protocol.TrainSetSpeedMemberAck{Addr: m.Addr, OK: true})
				continue
			}
		}

		if ShouldApplyStartDelay(
			commandSpeed, leadingWasAtStartSpeed, m.CurrentSpeed, m.Speed,
			m.StartDelayMs, m.AccelRampMs,
		) {
			startDelays = append(startDelays, startDelayJob{
				addr:    m.Addr,
				target:  m.Speed,
				forward: m.Forward,
				delayMs: m.StartDelayMs,
			})
			acks = append(acks, protocol.TrainSetSpeedMemberAck{Addr: m.Addr, OK: true})
			continue
		}

		if err := apply(ctx, m.Addr, m.Speed, m.Forward); err != nil {
			allOK = false
			acks = append(acks, protocol.TrainSetSpeedMemberAck{
				Addr:  m.Addr,
				OK:    false,
				Error: errors.CodeCommandStationError,
			})
			continue
		}
		acks = append(acks, protocol.TrainSetSpeedMemberAck{Addr: m.Addr, OK: true})
	}

	s.scheduleDelayed(ctx, trainID, apply, startDelays, ramps)
	return acks, allOK
}

func (s *TrainSpeedScheduler) scheduleDelayed(
	parentCtx context.Context,
	trainID uint,
	apply TrainMemberSpeedApply,
	startDelays []startDelayJob,
	ramps []speedRampJob,
) {
	if len(startDelays) == 0 && len(ramps) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(parentCtx)
	s.mu.Lock()
	s.pending[trainID] = cancel
	s.mu.Unlock()

	for _, job := range startDelays {
		go s.runStartDelay(ctx, apply, job)
	}
	for _, job := range ramps {
		go s.runSpeedRamp(ctx, apply, job)
	}
}

func (s *TrainSpeedScheduler) runStartDelay(
	ctx context.Context,
	apply TrainMemberSpeedApply,
	job startDelayJob,
) {
	time.Sleep(time.Duration(job.delayMs) * time.Millisecond)
	if ctx.Err() != nil {
		return
	}
	_ = apply(ctx, job.addr, job.target, job.forward)
}

func (s *TrainSpeedScheduler) runSpeedRamp(
	ctx context.Context,
	apply TrainMemberSpeedApply,
	job speedRampJob,
) {
	steps, intervalMs, ok := planSpeedRampFromCurrent(job.current, job.target, job.rampMs, job.maxSteps)
	if !ok {
		_ = apply(ctx, job.addr, job.target, job.forward)
		return
	}

	speeds := AccelRampSpeeds(job.current, job.target, steps)
	interval := time.Duration(intervalMs) * time.Millisecond
	for i, speed := range speeds {
		_ = apply(ctx, job.addr, speed, job.forward)
		if ctx.Err() != nil {
			return
		}
		if i < len(speeds)-1 {
			time.Sleep(interval)
		}
	}
}

func planSpeedRampFromCurrent(current, target uint8, rampMs, maxSteps int) (steps int, intervalMs int, ok bool) {
	if target > current {
		return PlanAccelRamp(current, target, rampMs, maxSteps)
	}
	return PlanBrakeRamp(current, target, rampMs, maxSteps)
}
