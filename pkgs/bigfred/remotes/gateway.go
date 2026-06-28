package remotes

import (
	"context"
	"fmt"
	"sync"

	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
	"github.com/sirupsen/logrus"
)

// GatewayFactory builds one inbound protocol listener for a command station.
type GatewayFactory func(ctx context.Context, cfg GatewayConfig) (RemoteProtocol, error)

// GatewayConfig wires shared inbound resources for protocol gateways.
type GatewayConfig struct {
	LayoutID         uint
	CommandStationID uint
	Coordinator      *Coordinator
	Store            *remotepairing.Store
	Drive            InboundDrivePort
	Log              *logrus.Logger
	// Protocol-specific options are passed via typed config in each gateway.
	Extra map[string]any
}

var (
	gatewayFactoriesMu sync.RWMutex
	gatewayFactories   = map[string]GatewayFactory{}
)

// RegisterGatewayFactory registers a protocol gateway factory by name.
func RegisterGatewayFactory(name string, factory GatewayFactory) {
	if name == "" || factory == nil {
		return
	}
	gatewayFactoriesMu.Lock()
	gatewayFactories[name] = factory
	gatewayFactoriesMu.Unlock()
}

// NewGateway returns a protocol listener by name.
func NewGateway(ctx context.Context, name string, cfg GatewayConfig) (RemoteProtocol, error) {
	gatewayFactoriesMu.RLock()
	factory, ok := gatewayFactories[name]
	gatewayFactoriesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("remotes: unknown gateway %q", name)
	}
	return factory(ctx, cfg)
}

// RegisteredGateways returns every registered protocol name.
func RegisteredGateways() []string {
	gatewayFactoriesMu.RLock()
	defer gatewayFactoriesMu.RUnlock()
	out := make([]string, 0, len(gatewayFactories))
	for name := range gatewayFactories {
		out = append(out, name)
	}
	return out
}
