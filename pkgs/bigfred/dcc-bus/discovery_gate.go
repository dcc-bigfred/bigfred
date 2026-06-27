package dccbus

import (
	"context"
	"sync"

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
	if g.needZ21 && !g.z21Bound {
		return
	}
	if g.needWT && !g.wtBound {
		return
	}
	g.started = true
	go g.d.runDiscovery(g.ctx)
}

func (d *Daemon) protocolListeningCallback(gate *discoveryGate, protocol string) func(context.Context) {
	return func(context.Context) {
		gate.markBound(protocol)
	}
}

func (d *Daemon) runDiscovery(ctx context.Context) {
	instance := discovery.InstanceName(d.cfg.CommandStation.Name, d.cfg.CommandStationID)
	var services []discovery.ServiceConfig
	if d.cfg.EnableZ21 {
		services = append(services, discovery.ServiceConfig{
			Protocol:         contract.RemoteProtocolZ21,
			LayoutID:         d.cfg.LayoutID,
			CommandStationID: d.cfg.CommandStationID,
			InstanceName:     instance,
			Port:             int(d.cfg.Z21Port),
		})
	}
	if d.cfg.EnableWithrottle {
		services = append(services, discovery.ServiceConfig{
			Protocol:         contract.RemoteProtocolWithrottle,
			LayoutID:         d.cfg.LayoutID,
			CommandStationID: d.cfg.CommandStationID,
			InstanceName:     instance,
			Port:             int(d.cfg.WithrottlePort),
		})
	}
	var beacon *discovery.Z21BeaconConfig
	if d.cfg.EnableZ21 {
		beacon = &discovery.Z21BeaconConfig{
			Port:             int(d.cfg.Z21Port),
			LayoutID:         d.cfg.LayoutID,
			CommandStationID: d.cfg.CommandStationID,
		}
	}
	if err := discovery.Run(ctx, discovery.RunConfig{
		Services:  services,
		Z21Beacon: beacon,
	}, d.log); err != nil && ctx.Err() == nil {
		d.log.WithError(err).Warn("handset discovery stopped")
	}
}
