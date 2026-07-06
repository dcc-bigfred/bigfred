package service

import (
	"errors"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const brakeRetryCount = 5

// DCCWriter issues SetSpeed commands to the command station with optional
// background retries for stop/emergency moves.
type DCCWriter struct {
	Station    commandstation.Station
	SpeedSteps uint
	Log        *logrus.Logger
	LogFields  func() logrus.Fields
}

// WireSpeedFromPayload maps a throttle payload speed to the DCC wire speed
// sent to the command station. DCC wire speed 1 is the emergency-brake step,
// so a normal (non-emergency) request for payload 1 — the first driving notch —
// is promoted to wire speed 2, the first real moving step. Emergency requests
// always emit wire speed 1; stop (payload 0) emits wire speed 0.
func WireSpeedFromPayload(payload uint8, emergency bool) uint8 {
	if emergency {
		return 1
	}
	if payload == 1 {
		return 2
	}
	return payload
}

// SetSpeed sends one SetSpeed to the command station. When the resulting wire
// speed is a stop/emergency-brake (0 or 1), a failed call is retried in a
// background goroutine so a dropped frame never leaves a loco moving.
func (w *DCCWriter) SetSpeed(addr uint16, payloadSpeed uint8, forward bool, emergency bool) error {
	wireSpeed := WireSpeedFromPayload(payloadSpeed, emergency)
	err := w.setSpeedOnStation(addr, wireSpeed, forward, emergency)
	if err == nil {
		return nil
	}
	if errors.Is(err, commandstation.ErrSpeedSuperseded) {
		return err
	}
	if wireSpeed <= 1 {
		go w.retrySetSpeed(addr, wireSpeed, forward, brakeRetryCount)
	}
	return err
}

func (w *DCCWriter) retrySetSpeed(addr uint16, wireSpeed uint8, forward bool, retries int) {
	fields := logrus.Fields{
		"addr":    addr,
		"speed":   wireSpeed,
		"forward": forward,
	}
	if w.LogFields != nil {
		for k, v := range w.LogFields() {
			fields[k] = v
		}
	}
	for i := 0; i < retries; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := w.setSpeedOnStation(addr, wireSpeed, forward, false); err != nil {
			if w.Log != nil {
				w.Log.WithError(err).WithFields(fields).WithField("attempt", i+1).Warn("dcc-bus SetSpeed retry failed")
			}
			continue
		}
		if w.Log != nil {
			w.Log.WithFields(fields).WithField("attempt", i+1).Info("dcc-bus SetSpeed retry succeeded")
		}
		return
	}
}

func (w *DCCWriter) setSpeedOnStation(addr uint16, wireSpeed uint8, forward bool, emergency bool) error {
	la := commandstation.LocoAddr(addr)
	if emergency {
		if estopper, ok := w.Station.(commandstation.EmergencyStopper); ok {
			return estopper.EmergencyStop(la, forward)
		}
	}
	return w.Station.SetSpeed(la, wireSpeed, forward, uint8(w.SpeedSteps))
}
