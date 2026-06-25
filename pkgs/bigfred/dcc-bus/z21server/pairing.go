package z21server

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

// PairingHandler buffers CV3/CV4 POM writes and completes Redis pairing.
type PairingHandler struct {
	store            *z21pairing.Store
	layoutID         uint
	commandStationID uint
	registry         *Registry
	onPaired         func(ctx context.Context)
}

// NewPairingHandler returns a handler for CV3/CV4 intercept.
func NewPairingHandler(store *z21pairing.Store, layoutID, commandStationID uint, registry *Registry, onPaired func(ctx context.Context)) *PairingHandler {
	return &PairingHandler{
		store:            store,
		layoutID:         layoutID,
		commandStationID: commandStationID,
		registry:         registry,
		onPaired:         onPaired,
	}
}

// Handle intercepts one CV3/CV4 POM write. Returns true when the packet was consumed.
func (h *PairingHandler) Handle(ctx context.Context, client *Client, cvWire, value int) (bool, *contract.Z21PairingActiveWire) {
	if h.store == nil {
		return true, nil
	}
	cv3, cv4, ready := client.BufferPairingCV(cvWire, value)
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
	cv3, cv4, ready := client.BufferPairingFn(fn)
	if !ready {
		return true, nil
	}
	return h.completePairing(ctx, client, cv3, cv4)
}

func (h *PairingHandler) completePairing(ctx context.Context, client *Client, cv3, cv4 int) (bool, *contract.Z21PairingActiveWire) {
	active, ok, err := h.store.PairViaCV3CV4(ctx, h.layoutID, h.commandStationID, cv3, cv4, client.Key, contract.NowMS())
	if err != nil || !ok {
		client.clearPairingBuffer()
		return true, nil
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
