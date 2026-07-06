package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
)

type capResponder struct {
	order  []uint16
	subs   map[uint16]struct{}
	errors []protocol.LocoErrorPayload
}

func newCapResponder() *capResponder {
	return &capResponder{subs: make(map[uint16]struct{}, 8)}
}

func (r *capResponder) Subscribe(addrs ...uint16) {
	for _, a := range addrs {
		if _, ok := r.subs[a]; ok {
			continue
		}
		r.subs[a] = struct{}{}
		r.order = append(r.order, a)
	}
}

func (r *capResponder) Unsubscribe(addrs ...uint16) {
	for _, a := range addrs {
		if _, ok := r.subs[a]; !ok {
			continue
		}
		delete(r.subs, a)
		out := r.order[:0]
		for _, x := range r.order {
			if x != a {
				out = append(out, x)
			}
		}
		r.order = out
	}
}

func (r *capResponder) SubscribedAddrs() []uint16 {
	out := make([]uint16, 0, len(r.subs))
	for a := range r.subs {
		out = append(out, a)
	}
	return out
}

func (r *capResponder) OldestSubscribed() (uint16, bool) {
	for _, a := range r.order {
		if _, ok := r.subs[a]; ok {
			return a, true
		}
	}
	return 0, false
}

func (r *capResponder) SelectedAddr() uint16                      { return 0 }
func (r *capResponder) SetSelected(uint16)                        {}
func (r *capResponder) ClearSelected()                            {}
func (r *capResponder) SendLocoState(context.Context, contract.LocoStateWire) error {
	return nil
}
func (r *capResponder) SendLocoError(_ context.Context, addr uint16, code, detail string) error {
	return r.SendLocoErrorPayload(context.Background(), protocol.LocoErrorPayload{
		Address: addr,
		Code:    code,
		Detail:  detail,
	})
}
func (r *capResponder) SendLocoErrorPayload(_ context.Context, p protocol.LocoErrorPayload) error {
	r.errors = append(r.errors, p)
	return nil
}
func (r *capResponder) SendAck(context.Context, string, protocol.AckPayload) error {
	return nil
}

func TestHandleSubscribeEnforcesCapDropOldest(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &slotStubStation{}
	r, err := NewRouter(context.Background(), Config{
		Station:            st,
		Hub:                &stubHub{},
		Redis:              rs,
		AllowedVehicles:    contract.AllowedVehicles{LayoutID: 1, Vehicles: vehicles(3, 7, 42)},
		LayoutID:           1,
		CommandStationID:   1,
		SpeedSteps:         128,
		MaxVehiclesPerUser: 2,
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	resp := newCapResponder()
	actor := Actor{UserID: 1, SessionID: "s1"}

	r.HandleSubscribe(context.Background(), actor, resp, protocol.LocoSubscribePayload{
		Addresses: []uint16{3, 7},
	}, "")
	if len(resp.SubscribedAddrs()) != 2 {
		t.Fatalf("subs = %v, want 2", resp.SubscribedAddrs())
	}

	r.HandleSubscribe(context.Background(), actor, resp, protocol.LocoSubscribePayload{
		Addresses: []uint16{42},
	}, "")

	if len(resp.SubscribedAddrs()) != 2 {
		t.Fatalf("subs after overflow = %v, want 2", resp.SubscribedAddrs())
	}
	if _, ok := resp.subs[3]; ok {
		t.Fatal("addr 3 should have been evicted as oldest")
	}
	if _, ok := resp.subs[42]; !ok {
		t.Fatal("addr 42 should be subscribed")
	}
	if len(resp.errors) != 1 || resp.errors[0].Code != errors.CodeSubscriptionCap || resp.errors[0].Address != 3 {
		t.Fatalf("errors = %+v, want subscription_cap for addr 3", resp.errors)
	}
}

func TestHandleSubscribeResubscribeDoesNotEvict(t *testing.T) {
	t.Parallel()
	rs, cleanup := testRedis(t)
	defer cleanup()
	st := &slotStubStation{}
	r, err := NewRouter(context.Background(), Config{
		Station:            st,
		Hub:                &stubHub{},
		Redis:              rs,
		AllowedVehicles:    contract.AllowedVehicles{LayoutID: 1, Vehicles: vehicles(3, 7)},
		LayoutID:           1,
		CommandStationID:   1,
		SpeedSteps:         128,
		MaxVehiclesPerUser: 1,
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	resp := newCapResponder()
	actor := Actor{UserID: 1, SessionID: "s1"}

	r.HandleSubscribe(context.Background(), actor, resp, protocol.LocoSubscribePayload{Addresses: []uint16{3}}, "")
	r.HandleSubscribe(context.Background(), actor, resp, protocol.LocoSubscribePayload{Addresses: []uint16{3}}, "")

	if len(resp.SubscribedAddrs()) != 1 {
		t.Fatalf("subs = %v, want 1", resp.SubscribedAddrs())
	}
	if len(resp.errors) != 0 {
		t.Fatalf("unexpected errors: %+v", resp.errors)
	}
}

func vehicles(addrs ...uint16) []contract.AllowedVehicle {
	out := make([]contract.AllowedVehicle, len(addrs))
	for i, a := range addrs {
		out[i] = contract.AllowedVehicle{Addr: a}
	}
	return out
}
