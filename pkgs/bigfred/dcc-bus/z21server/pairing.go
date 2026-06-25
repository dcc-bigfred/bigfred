package z21server

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

// PairingHandler buffers CV3/CV4 POM writes and completes Redis pairing.
type PairingHandler struct {
	store            *z21pairing.Store
	layoutID         uint
	commandStationID uint
	registry         *Registry
}

// NewPairingHandler returns a handler for CV3/CV4 intercept.
func NewPairingHandler(store *z21pairing.Store, layoutID, commandStationID uint, registry *Registry) *PairingHandler {
	return &PairingHandler{
		store:            store,
		layoutID:         layoutID,
		commandStationID: commandStationID,
		registry:         registry,
	}
}

// Handle intercepts one POM write. Returns true when the packet was consumed.
func (h *PairingHandler) Handle(ctx context.Context, client *Client, cvWire, value int) (bool, *contract.Z21PairingActiveWire) {
	if h.store == nil {
		return true, nil
	}
	cv3, cv4, ready := client.BufferPairingCV(cvWire, value)
	if !ready {
		return true, nil
	}
	active, ok, err := h.store.PairViaCV3CV4(ctx, h.layoutID, h.commandStationID, cv3, cv4, client.Key, contract.NowMS())
	if err != nil || !ok {
		client.clearPairingBuffer()
		return true, nil
	}
	h.registry.SetPaired(client.Key, &active)
	return true, &active
}
