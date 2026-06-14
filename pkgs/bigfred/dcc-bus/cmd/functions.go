package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// pulseRetryInterval is the pause between timed-pulse SendFn retries.
	pulseRetryInterval = 100 * time.Millisecond
)

// markTimedFunctionStarted marks a timed function pulse as in-flight. false means
// another pulse for the same (addr, fn) is still running.
func (r *Router) markTimedFunctionStarted(key fnKey) bool {
	r.pulseMu.Lock()
	defer r.pulseMu.Unlock()
	active := r.pulseActive[key]
	r.pulseActive[key] = true
	return !active
}

// markTimedFunctionStopped marks a timed function pulse as completed.
func (r *Router) markTimedFunctionStopped(key fnKey) {
	r.pulseMu.Lock()
	delete(r.pulseActive, key)
	r.pulseMu.Unlock()
}

// setLocoFunctionWithRetry calls setLocoFunction up to retry+1 times.
// retry 0 means a single attempt.
func (r *Router) setLocoFunctionWithRetry(ctx context.Context, addr uint16, userID uint, fn uint8, on bool, source string, retry int) error {
	attempts := retry + 1
	var last error
	for i := 0; i < attempts; i++ {
		last = r.setLocoFunction(ctx, addr, userID, fn, on, source)
		if last == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(pulseRetryInterval)
		}
	}
	return last
}

// setTimedLocoFunctionWithRetry turns fn on, then off after duration.
// source is recorded in Redis state snapshots (e.g. "deadman", "script").
// Overlapping calls for the same (addr, fn) are ignored until the
// previous pulse completes (including the off phase). retry is the
// number of extra attempts after the first on each phase (0 = none).
func (r *Router) setTimedLocoFunctionWithRetry(addr uint16, userID uint, fn uint8, duration time.Duration, source string, retry int) {
	key := fnKey{Addr: addr, Fn: fn}
	if !r.markTimedFunctionStarted(key) {
		r.log.WithFields(logrus.Fields{
			"addr":     addr,
			"function": fn,
			"source":   source,
		}).Debug("dcc-bus pulse function skipped: already active")
		return
	}
	go func() {
		ctx := context.Background()
		if err := r.setLocoFunctionWithRetry(ctx, addr, userID, fn, true, source, retry); err != nil {
			r.markTimedFunctionStopped(key)
			r.log.WithError(err).WithFields(logrus.Fields{
				"addr":     addr,
				"function": fn,
				"source":   source,
				"duration": duration,
				"retry":    retry,
			}).Warn("dcc-bus pulse function on failed")
			return
		}
		time.AfterFunc(duration, func() {
			defer r.markTimedFunctionStopped(key)
			if err := r.setLocoFunctionWithRetry(context.Background(), addr, userID, fn, false, source, retry); err != nil {
				r.log.WithError(err).WithFields(logrus.Fields{
					"addr":     addr,
					"function": fn,
					"source":   source,
					"retry":    retry,
				}).Warn("dcc-bus pulse function off failed")
			}
		})
	}()
}
