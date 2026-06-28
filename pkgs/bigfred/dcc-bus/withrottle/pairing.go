package withrottle

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

// PairingHandler matches N-code and F-key sequences and completes Redis pairing.
type PairingHandler struct {
	store            *remotepairing.Store
	layoutID         uint
	commandStationID uint
	registry         *Registry
	onPaired         func(ctx context.Context, key string, active *contract.RemoteSessionWire)
	onEvictedClient  func(ctx context.Context, clientKey string)
}

// NewPairingHandler returns a handler for WiThrottle pairing entry paths.
func NewPairingHandler(store *remotepairing.Store, layoutID, commandStationID uint, registry *Registry, onPaired func(ctx context.Context, key string, active *contract.RemoteSessionWire), onEvictedClient func(ctx context.Context, clientKey string)) *PairingHandler {
	return &PairingHandler{
		store:            store,
		layoutID:         layoutID,
		commandStationID: commandStationID,
		registry:         registry,
		onPaired:         onPaired,
		onEvictedClient:  onEvictedClient,
	}
}

// HandleN attempts device-name pairing from an N line. Returns true when consumed.
func (h *PairingHandler) HandleN(ctx context.Context, client *Client, name string) (bool, *contract.RemoteSessionWire) {
	if h.store == nil || h.registry.IsPaired(client.Key) {
		return false, nil
	}
	code := contract.NormalizeWithrottleDeviceName(name)
	if !contract.ValidWithrottleCode(code) {
		return false, nil
	}
	return h.completePairing(ctx, client, code)
}

// HandleFn intercepts one function-key ON while unpaired on the sentinel.
func (h *PairingHandler) HandleFn(ctx context.Context, client *Client, fn int) (bool, *contract.RemoteSessionWire) {
	if h.store == nil {
		return false, nil
	}
	code, ready := h.registry.BufferPairingFn(client.Key, fn)
	if !ready {
		return true, nil
	}
	return h.completePairing(ctx, client, code)
}

func (h *PairingHandler) completePairing(ctx context.Context, client *Client, code string) (bool, *contract.RemoteSessionWire) {
	active, ok, evicted, err := h.store.PairViaWithrottleCode(ctx, h.layoutID, h.commandStationID, code, client.Key, contract.NowMS())
	if err != nil || !ok {
		h.registry.ClearPairingBuffer(client.Key)
		return true, nil
	}
	if evicted != "" && h.onEvictedClient != nil && evicted != client.Key {
		h.onEvictedClient(ctx, evicted)
	}
	h.registry.SetPaired(client.Key, &active)
	if h.onPaired != nil {
		h.onPaired(ctx, client.Key, &active)
	}
	return true, &active
}

func pairingLogFields(active *contract.RemoteSessionWire) logrus.Fields {
	if active == nil {
		return logrus.Fields{}
	}
	fields := logrus.Fields{"userId": active.UserID}
	if active.AllowAllVehicles {
		fields["allowedLocos"] = "all"
		return fields
	}
	fields["allowedAddrs"] = active.AllowedAddrs
	return fields
}
