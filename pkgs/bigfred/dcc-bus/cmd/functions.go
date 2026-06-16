package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const pulseRetryInterval = 100 * time.Millisecond

func (r *Router) markTimedFunctionStarted(key service.FnKey) bool {
	r.pulseMu.Lock()
	defer r.pulseMu.Unlock()
	active := r.pulseActive[key]
	r.pulseActive[key] = true
	return !active
}

func (r *Router) markTimedFunctionStopped(key service.FnKey) {
	r.pulseMu.Lock()
	delete(r.pulseActive, key)
	r.pulseMu.Unlock()
}

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

func (r *Router) setLocoFunction(ctx context.Context, addr uint16, userID uint, fn uint8, on bool, source string) error {
	if r.checkFnStateMatches(ctx, addr, fn, on) {
		return nil
	}
	previous, hadPrev := r.cache.Stage(addr, fn, on)
	if err := r.station.SendFn(commandstation.MainTrackMode, commandstation.LocoAddr(addr), commandstation.FuncNum(fn), on); err != nil {
		fields := r.stationLogFields()
		fields["addr"] = addr
		fields["function"] = fn
		fields["on"] = on
		r.log.WithError(err).WithFields(fields).Warn("dcc-bus command station SendFn failed")
		r.cache.Rollback(addr, fn, previous, hadPrev)
		return err
	}

	snap := contract.LocoStateWire{
		Address:            addr,
		ControlledByUserID: userID,
		Source:             source,
		At:                 time.Now().UTC().UnixMilli(),
	}
	if env, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
		snap.Speed = env.Speed
		snap.Forward = env.Forward
		snap.Functions = make([]bool, maxInt(len(env.Functions), int(fn)+1))
		copy(snap.Functions, env.Functions)
	} else {
		snap.Functions = make([]bool, int(fn)+1)
	}
	snap.Functions[fn] = on
	if err := r.redis.StoreState(ctx, snap, StateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus redis store")
	}
	service.BroadcastLocoState(ctx, r.hub, snap)
	return nil
}

func (r *Router) setTimedLocoFunctionWithRetry(addr uint16, userID uint, fn uint8, duration time.Duration, source string, retry int) {
	key := service.FnKey{Addr: addr, Fn: fn}
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

func (r *Router) checkFnStateMatches(ctx context.Context, addr uint16, fn uint8, on bool) bool {
	if !r.cache.Matches(addr, fn, on) {
		return false
	}
	env, ok, err := r.redis.LoadState(ctx, addr)
	if err != nil || !ok {
		return false
	}
	if int(fn) >= len(env.Functions) {
		return !on
	}
	return env.Functions[fn] == on
}
