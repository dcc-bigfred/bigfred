package dccbus

import (
	"context"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/discovery"
)

type discoveryGate struct {
	mu       sync.Mutex
	needZ21  bool
	needWT   bool
	z21Bound bool
	wtBound  bool
	started  bool
	ctx      context.Context
	d        *Daemon
}

func (d *Daemon) newDiscoveryGate(ctx context.Context) *discoveryGate {
	return &discoveryGate{
		needZ21: d.cfg.EnableZ21,
		needWT:  d.cfg.EnableWithrottle,
		ctx:     ctx,
		d:       d,
	}
}

func (g *discoveryGate) markBound(protocol string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch protocol {
	case contract.RemoteProtocolZ21:
		g.z21Bound = true
	case contract.RemoteProtocolWithrottle:
		g.wtBound = true
	default:
		return
	}
	if g.started {
		return
	}
	ready := (g.needZ21 && g.z21Bound) || (g.needWT && g.wtBound)
	if !ready {
		return
	}
	g.started = true
	z21 := g.z21Bound
	wt := g.wtBound
	go g.d.runDiscovery(g.ctx, z21, wt)
}

func (d *Daemon) protocolListeningCallback(gate *discoveryGate, protocol string) func(context.Context) {
	return func(context.Context) {
		gate.markBound(protocol)
	}
}

func (d *Daemon) runDiscovery(ctx context.Context, z21Bound, wtBound bool) {
	instance := discovery.InstanceName(d.cfg.CommandStation.Name, d.cfg.CommandStationID)
	for ctx.Err() == nil {
		var services []discovery.ServiceConfig
		if d.cfg.EnableZ21 && z21Bound {
			services = append(services, discovery.ServiceConfig{
				Protocol:         contract.RemoteProtocolZ21,
				LayoutID:         d.cfg.LayoutID,
				CommandStationID: d.cfg.CommandStationID,
				InstanceName:     instance,
				Port:             int(d.cfg.Z21Port),
			})
		}
		if d.cfg.EnableWithrottle && wtBound {
			services = append(services, discovery.ServiceConfig{
				Protocol:         contract.RemoteProtocolWithrottle,
				LayoutID:         d.cfg.LayoutID,
				CommandStationID: d.cfg.CommandStationID,
				InstanceName:     instance,
				Port:             int(d.cfg.WithrottlePort),
			})
		}
		if len(services) == 0 {
			return
		}
		var beacon *discovery.Z21BeaconConfig
		if d.cfg.EnableZ21 && z21Bound {
			beacon = &discovery.Z21BeaconConfig{
				Port:             int(d.cfg.Z21Port),
				LayoutID:         d.cfg.LayoutID,
				CommandStationID: d.cfg.CommandStationID,
			}
		}
		err := discovery.Run(ctx, discovery.RunConfig{
			Services:  services,
			Z21Beacon: beacon,
		}, d.log)
		if err == nil || ctx.Err() != nil {
			return
		}
		d.log.WithError(err).Warn("handset discovery stopped; retrying")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
