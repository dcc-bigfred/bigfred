package service_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
)

func TestCancelAll_abortsPendingTrainJobs(t *testing.T) {
	t.Parallel()

	sched := service.NewTrainSpeedScheduler()
	ctx := context.Background()
	var applyCalls atomic.Int32

	_, _ = sched.Apply(ctx, "train-a", 10, true, func(_ context.Context, _ uint16, _ uint8, _ bool) error {
		applyCalls.Add(1)
		return nil
	}, []service.TrainMemberSetSpeed{{
		Addr:         3,
		CurrentSpeed: 0,
		Speed:        10,
		Forward:      true,
		StartDelayMs: 5000,
	}})

	deadline := time.Now().Add(200 * time.Millisecond)
	for applyCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if applyCalls.Load() != 0 {
		t.Fatal("start delay should not have fired yet")
	}

	sched.CancelAll()

	time.Sleep(100 * time.Millisecond)
	if applyCalls.Load() != 0 {
		t.Fatalf("apply calls after CancelAll = %d, want 0", applyCalls.Load())
	}
}

func TestIsStartDelayPreviousSpeed(t *testing.T) {
	for _, speed := range []uint8{0, 1} {
		if !service.IsStartDelayPreviousSpeed(speed) {
			t.Fatalf("speed %d should qualify for consist start", speed)
		}
	}
	for _, speed := range []uint8{2, 3, 10, 127} {
		if service.IsStartDelayPreviousSpeed(speed) {
			t.Fatalf("speed %d should not qualify for consist start", speed)
		}
	}
}

func TestShouldApplyBrakeRamp(t *testing.T) {
	if !service.ShouldApplyBrakeRamp(10, 5, 5000) {
		t.Fatal("should brake ramp when decelerating")
	}
	if service.ShouldApplyBrakeRamp(5, 10, 5000) {
		t.Fatal("no brake ramp when accelerating")
	}
	if !service.ShouldApplyBrakeRamp(10, 0, 5000) {
		t.Fatal("stop to zero should brake ramp")
	}
	if service.ShouldApplyBrakeRamp(10, 5, 0) {
		t.Fatal("zero brake ramp ms should not schedule")
	}
}

func TestShouldApplyAccelRamp(t *testing.T) {
	if service.ShouldApplyAccelRamp(10, 0, 10, 0, 0) {
		t.Fatal("zero ramp ms should not schedule")
	}
	if !service.ShouldApplyAccelRamp(10, 0, 10, 0, 5000) {
		t.Fatal("should ramp when start delay disabled")
	}
	if service.ShouldApplyAccelRamp(10, 0, 10, 150, 5000) {
		t.Fatal("start delay enabled at standstill should block accel ramp")
	}
	if !service.ShouldApplyAccelRamp(10, 5, 10, 150, 5000) {
		t.Fatal("accel ramp should apply above start speed even with start delay set")
	}
	if service.ShouldApplyAccelRamp(0, 0, 0, 0, 5000) {
		t.Fatal("no ramp on stop")
	}
}

func TestShouldApplyStartDelay(t *testing.T) {
	if !service.ShouldApplyStartDelay(10, true, 0, 10, 150, 5000) {
		t.Fatal("should start-delay on consist start when accel is blocked")
	}
	if service.ShouldApplyStartDelay(10, true, 0, 10, 0, 5000) {
		t.Fatal("no start delay when disabled")
	}
	if service.ShouldApplyStartDelay(10, true, 0, 10, 150, 5000) &&
		service.ShouldApplyAccelRamp(10, 0, 10, 150, 5000) {
		t.Fatal("start delay and accel ramp must not both apply")
	}
	if service.ShouldApplyStartDelay(10, false, 0, 10, 150, 0) {
		t.Fatal("no start delay while consist already moving")
	}
	if service.ShouldApplyStartDelay(10, true, 5, 10, 150, 0) {
		t.Fatal("no start delay above start speed")
	}
}

func TestPlanAccelRamp(t *testing.T) {
	steps, interval, ok := service.PlanAccelRamp(0, 10, 5000, 2)
	if !ok || steps != 2 || interval != 2500 {
		t.Fatalf("PlanAccelRamp() = %d, %d, %v; want 2, 2500, true", steps, interval, ok)
	}

	steps, interval, ok = service.PlanAccelRamp(0, 10, 5000, 20)
	if !ok || steps != 10 || interval != 500 {
		t.Fatalf("PlanAccelRamp() = %d, %d, %v; want 10, 500, true", steps, interval, ok)
	}

	_, _, ok = service.PlanAccelRamp(5, 10, 5000, 2)
	if !ok {
		t.Fatal("expected ramp when accelerating")
	}
	_, _, ok = service.PlanAccelRamp(10, 5, 5000, 2)
	if ok {
		t.Fatal("expected no ramp when decelerating")
	}
}

func TestPlanBrakeRamp(t *testing.T) {
	steps, interval, ok := service.PlanBrakeRamp(10, 0, 5000, 2)
	if !ok || steps != 2 || interval != 2500 {
		t.Fatalf("PlanBrakeRamp() = %d, %d, %v; want 2, 2500, true", steps, interval, ok)
	}
	_, _, ok = service.PlanBrakeRamp(0, 10, 5000, 2)
	if ok {
		t.Fatal("expected no brake ramp when accelerating")
	}
}

func TestPlanAccelRampStepDelta(t *testing.T) {
	tests := []struct {
		name    string
		current uint8
		target  uint8
		steps   int
		perStep int
		sign    int
	}{
		{name: "accelerate even split", current: 0, target: 10, steps: 2, perStep: 5, sign: 1},
		{name: "accelerate remainder in last step", current: 0, target: 10, steps: 3, perStep: 3, sign: 1},
		{name: "decelerate", current: 10, target: 7, steps: 3, perStep: 1, sign: -1},
		{name: "single step", current: 2, target: 8, steps: 1, perStep: 6, sign: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			perStep, sign := service.PlanAccelRampStepDelta(tc.current, tc.target, tc.steps)
			if perStep != tc.perStep || sign != tc.sign {
				t.Fatalf("PlanAccelRampStepDelta() = %d, %d; want %d, %d",
					perStep, sign, tc.perStep, tc.sign)
			}
		})
	}
}

func TestAccelRampSpeeds(t *testing.T) {
	tests := []struct {
		name    string
		current uint8
		target  uint8
		steps   int
		want    []uint8
	}{
		{name: "two steps", current: 0, target: 10, steps: 2, want: []uint8{5, 10}},
		{name: "three steps", current: 0, target: 10, steps: 3, want: []uint8{3, 6, 10}},
		{name: "decelerate", current: 10, target: 7, steps: 3, want: []uint8{9, 8, 7}},
		{name: "single step", current: 4, target: 9, steps: 1, want: []uint8{9}},
		{name: "no steps falls back to target", current: 4, target: 9, steps: 0, want: []uint8{9}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := service.AccelRampSpeeds(tc.current, tc.target, tc.steps)
			if len(got) != len(tc.want) {
				t.Fatalf("AccelRampSpeeds() = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("AccelRampSpeeds() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
