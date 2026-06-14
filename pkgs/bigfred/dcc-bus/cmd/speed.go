package cmd

import (
	"time"

	"github.com/keskad/loco/pkgs/loco/commandstation"
	"github.com/sirupsen/logrus"
)

const brakeRetryCount = 5

// stationSetSpeed sends one SetSpeed to the command station. When payloadSpeed
// is 0 or emergency is true, a failed call is retried up to speedRetryCount
// times in a background goroutine.
func (r *Router) stationSetSpeed(addr uint16, payloadSpeed uint8, forward bool, emergency bool) error {
	wireSpeed := payloadSpeed
	if emergency {
		wireSpeed = 1 // DCC EMG-stop is "speed step 1" in 128-step mode
	}
	err := r.station.SetSpeed(commandstation.LocoAddr(addr), wireSpeed, forward, uint8(r.speedSteps))
	if err == nil {
		return nil
	}

	// we need to make sure the loco is stopped. In case of emergencies, we need to make sure the operator is able to stop the loco
	// in other cases we don't want to cause delayed reaction times.
	if payloadSpeed <= 1 || emergency {
		go r.retryStationSetSpeed(addr, wireSpeed, forward, brakeRetryCount)
	}

	return err
}

// retryStationSetSpeed retries a SetSpeed command up to speedRetryCount times.
func (r *Router) retryStationSetSpeed(addr uint16, wireSpeed uint8, forward bool, speedRetryCount int) {
	fields := logrus.Fields{
		"addr":    addr,
		"speed":   wireSpeed,
		"forward": forward,
	}
	for i := 0; i < speedRetryCount; i++ {
		time.Sleep(time.Millisecond * 100)
		if err := r.station.SetSpeed(commandstation.LocoAddr(addr), wireSpeed, forward, uint8(r.speedSteps)); err != nil {
			r.log.WithError(err).WithFields(fields).WithField("attempt", i+1).Warn("dcc-bus SetSpeed retry failed")
			continue
		}
		r.log.WithFields(fields).WithField("attempt", i+1).Info("dcc-bus SetSpeed retry succeeded")
		return
	}
}
