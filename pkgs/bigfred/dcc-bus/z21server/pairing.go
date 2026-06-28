package z21server

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

// PairingHandler buffers CV3/CV4 POM writes and completes Redis pairing.
type PairingHandler struct {
	store            *remotepairing.Store
	layoutID         uint
	commandStationID uint
	registry         *Registry
	onPaired         func(ctx context.Context)
	onEvictedClient  func(ctx context.Context, clientKey string)
}

// NewPairingHandler returns a handler for CV3/CV4 intercept. onEvictedClient
// is invoked when completing pairing evicted a prior session for the same user
// so the caller can clean up that client's in-process registry/wire state.
func NewPairingHandler(store *remotepairing.Store, layoutID, commandStationID uint, registry *Registry, onPaired func(ctx context.Context), onEvictedClient func(ctx context.Context, clientKey string)) *PairingHandler {
	return &PairingHandler{
		store:            store,
		layoutID:         layoutID,
		commandStationID: commandStationID,
		registry:         registry,
		onPaired:         onPaired,
		onEvictedClient:  onEvictedClient,
	}
}

// Handle intercepts one CV3/CV4 POM write. Returns true when the packet was consumed.
func (h *PairingHandler) Handle(ctx context.Context, client *Client, cvWire, value int) (bool, *contract.Z21PairingActiveWire) {
	if h.store == nil {
		return true, nil
	}
	cv3, cv4, ready := h.registry.BufferPairingCV(client.Key, cvWire, value)
	if !ready {
		return true, nil
	}
	return h.completePairing(ctx, client, cv3, cv4)
}

// HandleFn intercepts one function-key ON while unpaired. Returns true when the
// press was consumed for pairing entry (including incomplete codes).
func (h *PairingHandler) HandleFn(ctx context.Context, client *Client, fn int) (bool, *contract.Z21PairingActiveWire) {
	if h.store == nil {
		return false, nil
	}
	cv3, cv4, ready := h.registry.BufferPairingFn(client.Key, fn)
	if !ready {
		return true, nil
	}
	return h.completePairing(ctx, client, cv3, cv4)
}

func (h *PairingHandler) completePairing(ctx context.Context, client *Client, cv3, cv4 int) (bool, *contract.Z21PairingActiveWire) {
	active, ok, evicted, err := h.store.PairViaCV3CV4(ctx, h.layoutID, h.commandStationID, cv3, cv4, client.Key, contract.NowMS())
	if err != nil || !ok {
		h.registry.ClearPairingBuffer(client.Key)
		return true, nil
	}
	// Pairing evicted a prior handset for the same user (re-pair on a new
	// IP:port). Drop the old client's in-process state immediately so it
	// cannot keep driving until its next packet/self-heal.
	if evicted != "" && h.onEvictedClient != nil && evicted != client.Key {
		h.onEvictedClient(ctx, evicted)
	}
	h.registry.SetPaired(client.Key, &active)
	if h.onPaired != nil {
		h.onPaired(ctx)
	}
	return true, &active
}

func pairingLogFields(active *contract.Z21PairingActiveWire) logrus.Fields {
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