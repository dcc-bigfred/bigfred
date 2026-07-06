// Package sim runs the sustained drive pattern for each configured vehicle.
package sim

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/loadtest/dccbus"
	"github.com/keskad/loco/pkgs/bigfred/loadtest/httpapi"
)

const maxFunction = uint8(28) // LocoNet driver supports F0–F28

// Bus is the subset of dcc-bus operations the simulator needs.
type Bus interface {
	Select(ctx context.Context, address uint16) error
	SetSpeed(ctx context.Context, address uint16, speed uint8, forward bool) error
	SetFunction(ctx context.Context, address uint16, fn uint8, on bool) error
}

// Driver orchestrates per-vehicle simulation loops.
type Driver struct {
	bus  Bus
	log  *logrus.Logger
	cfg  Config
}

// New returns a Driver backed by the given dcc-bus client.
func New(bus Bus, log *logrus.Logger, cfg Config) *Driver {
	if log == nil {
		log = logrus.New()
	}
	return &Driver{
		bus: bus,
		log: log,
		cfg: cfg,
	}
}

// Run starts one simulation goroutine per loco and blocks until ctx is
// cancelled or a simulation goroutine fails. Teardown sets speed to 0 and
// turns functions off.
func (d *Driver) Run(ctx context.Context, locos []httpapi.Loco) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, len(locos))

	for _, loco := range locos {
		wg.Add(1)
		go func(l httpapi.Loco) {
			defer wg.Done()
			if err := d.runLoco(runCtx, l); err != nil && runCtx.Err() == nil {
				errCh <- fmt.Errorf("%s: %w", l.VehicleID, err)
				cancel()
			}
		}(loco)
	}

	simDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(simDone)
	}()

	select {
	case err := <-errCh:
		d.log.WithError(err).Error("simulation failed")
		<-simDone
		teardownCtx, teardownCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer teardownCancel()
		d.teardown(teardownCtx, locos)
		return err
	case <-ctx.Done():
	}

	d.log.Info("shutting down simulation goroutines before teardown")
	// Cancel simulation goroutines and wait for them to exit fully BEFORE
	// issuing teardown commands. Running teardown concurrently with live
	// runLoco goroutines creates a race: a goroutine whose shuttle timer
	// fires after teardown already sent speed=0 for that address will call
	// SetSpeed(maxSpeed) with a higher generation counter, causing the
	// LocoNet driver to coalesce (drop) the teardown's speed=0 frame and
	// send maxSpeed instead. The loco then leaves the bus at full speed.
	cancel()
	<-simDone

	d.log.Info("running teardown")
	// Budget: 19 locos × 30 commands × ~10 ms each ≈ 5.7 s; 60 s gives ample
	// headroom when the bus is slower than nominal or more locos are driven.
	teardownCtx, teardownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer teardownCancel()
	d.teardown(teardownCtx, locos)
	return ctx.Err()
}

func (d *Driver) runLoco(ctx context.Context, loco httpapi.Loco) error {
	log := d.log.WithFields(logrus.Fields{
		"vehicleId": loco.VehicleID,
		"address":   loco.Address,
	})

	startupFns := []uint8{0}
	if !d.cfg.WithoutF1 {
		startupFns = append(startupFns, 1)
	}
	for _, fn := range startupFns {
		if err := d.bus.SetFunction(ctx, loco.Address, fn, true); err != nil {
			return fmt.Errorf("enable F%d: %w", fn, err)
		}
	}
	if d.cfg.WithoutF1 {
		log.Info("enabled F0")
	} else {
		log.Info("enabled F0 and F1")
	}

	forward := true
	if err := d.bus.Select(ctx, loco.Address); err != nil {
		return fmt.Errorf("loco.select: %w", err)
	}
	if err := d.bus.SetSpeed(ctx, loco.Address, d.cfg.MaxSpeed, forward); err != nil {
		return fmt.Errorf("set initial speed: %w", err)
	}
	log.WithFields(logrus.Fields{
		"speed":   d.cfg.MaxSpeed,
		"forward": forward,
	}).Info("shuttle started")

	rng := rand.New(rand.NewSource(time.Now().UnixNano() ^ int64(loco.Address)<<16))

	shuttleTimer := time.NewTimer(d.cfg.LegDuration)
	defer shuttleTimer.Stop()

	hornTimer := time.NewTimer(randomInterval(rng, d.cfg.HornMinInterval, d.cfg.HornMaxInterval))
	defer hornTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-shuttleTimer.C:
			forward = !forward
			if err := d.bus.SetSpeed(ctx, loco.Address, d.cfg.MaxSpeed, forward); err != nil {
				return fmt.Errorf("set speed %d forward=%v: %w", d.cfg.MaxSpeed, forward, err)
			}
			log.WithFields(logrus.Fields{
				"speed":   d.cfg.MaxSpeed,
				"forward": forward,
			}).Debug("shuttle direction changed")
			resetTimer(shuttleTimer, d.cfg.LegDuration)

		case <-hornTimer.C:
			if err := d.pulseFunction(ctx, loco.Address, d.cfg.HornFunction); err != nil {
				return fmt.Errorf("horn F%d: %w", d.cfg.HornFunction, err)
			}
			log.WithField("function", d.cfg.HornFunction).Debug("horn pulsed")
			resetTimer(hornTimer, randomInterval(rng, d.cfg.HornMinInterval, d.cfg.HornMaxInterval))
		}
	}
}

func (d *Driver) pulseFunction(ctx context.Context, address uint16, fn uint8) error {
	if err := d.bus.SetFunction(ctx, address, fn, true); err != nil {
		return fmt.Errorf("enable F%d: %w", fn, err)
	}

	timer := time.NewTimer(d.cfg.HornPulseDuration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		_ = d.bus.SetFunction(context.Background(), address, fn, false)
		return nil
	case <-timer.C:
	}

	if err := d.bus.SetFunction(ctx, address, fn, false); err != nil {
		return fmt.Errorf("disable F%d: %w", fn, err)
	}
	return nil
}

func (d *Driver) teardown(ctx context.Context, locos []httpapi.Loco) {
	for _, loco := range locos {
		if err := d.bus.SetSpeed(ctx, loco.Address, 0, true); err != nil {
			d.log.WithError(err).WithField("vehicleId", loco.VehicleID).Warn("teardown set speed failed")
		}
		for fn := uint8(0); fn <= maxFunction; fn++ {
			if ctx.Err() != nil {
				d.log.WithFields(logrus.Fields{
					"vehicleId":   loco.VehicleID,
					"lastFunction": fn,
				}).Warn("teardown context expired, remaining functions may stay on")
				break
			}
			if err := d.bus.SetFunction(ctx, loco.Address, fn, false); err != nil {
				d.log.WithError(err).WithFields(logrus.Fields{
					"vehicleId": loco.VehicleID,
					"function":  fn,
				}).Warn("teardown set function failed, continuing")
				// continue to attempt next functions rather than leaving them active
			}
		}
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

func randomInterval(rng *rand.Rand, min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	delta := max - min
	return min + time.Duration(rng.Int63n(int64(delta)+1))
}

// Ensure dccbus.Client satisfies Bus at compile time.
var _ Bus = (*dccbus.Client)(nil)
