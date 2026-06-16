package service

import (
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

// SetSpeed sends one SetSpeed to the command station. When payloadSpeed is 0
// or emergency is true, a failed call is retried in a background goroutine.
func (w *DCCWriter) SetSpeed(addr uint16, payloadSpeed uint8, forward bool, emergency bool) error {
	wireSpeed := payloadSpeed
	if emergency {
		wireSpeed = 1
	}
	err := w.Station.SetSpeed(commandstation.LocoAddr(addr), wireSpeed, forward, uint8(w.SpeedSteps))
	if err == nil {
		return nil
	}
	if payloadSpeed <= 1 || emergency {
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
		if err := w.Station.SetSpeed(commandstation.LocoAddr(addr), wireSpeed, forward, uint8(w.SpeedSteps)); err != nil {
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

// UISpeedFromWire maps a command-station speed reading to the UI snapshot
// value. Wire speed 1 is DCC EMG-stop; halted locos are always exposed as 0.
func UISpeedFromWire(wire uint8) uint8 {
	if wire == 1 {
		return 0
	}
	return wire
}
