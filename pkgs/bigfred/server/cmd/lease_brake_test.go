package cmd

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	dccprotocol "github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

type recordingDccBus struct {
	calls []recordedPublish
}

type recordedPublish struct {
	layoutID         uint
	commandStationID uint
	typ              string
	payload          any
}

func (r *recordingDccBus) PublishCommand(_ context.Context, layoutID, commandStationID uint, typ string, payload any) error {
	r.calls = append(r.calls, recordedPublish{layoutID, commandStationID, typ, payload})
	return nil
}

type stubLeaseBrakeLayouts struct {
	csIDs []uint
}

func (s stubLeaseBrakeLayouts) CommandStationIDsForLayout(context.Context, uint) ([]uint, error) {
	return s.csIDs, nil
}

func TestLeaseBrakeStopLeasedTarget_publishesEStopTarget(t *testing.T) {
	t.Parallel()
	addr := uint16(12)
	vehicleID, err := domain.NewVehicleID()
	if err != nil {
		t.Fatal(err)
	}
	bus := &recordingDccBus{}
	brake := NewLeaseBrake(LeaseBrakeConfig{
		DccBus: bus,
		Roster: stubDriveTargetRoster{
			vehicles: []RosterVehicleEntry{{
				Vehicle: domain.Vehicle{ID: vehicleID, DCCAddress: ptrUint16(addr)},
			}},
		},
		Layouts: stubLeaseBrakeLayouts{csIDs: []uint{3}},
	})
	if err := brake.StopLeasedTarget(context.Background(), 2, domain.TakeoverTargetVehicle, vehicleID.String()); err != nil {
		t.Fatal(err)
	}
	if len(bus.calls) != 1 {
		t.Fatalf("publish calls = %d, want 1", len(bus.calls))
	}
	call := bus.calls[0]
	if call.layoutID != 2 || call.commandStationID != 3 || call.typ != dccprotocol.TypeSystemEStopTarget {
		t.Fatalf("unexpected publish: %+v", call)
	}
	wire, ok := call.payload.(contract.EStopTargetCommandWire)
	if !ok {
		t.Fatalf("payload type %T", call.payload)
	}
	if len(wire.Addresses) != 1 || wire.Addresses[0] != addr {
		t.Fatalf("addresses = %v, want [%d]", wire.Addresses, addr)
	}
}

type flakyDccBus struct {
	failOn map[uint]bool
	calls  []uint
}

func (f *flakyDccBus) PublishCommand(_ context.Context, _, commandStationID uint, _ string, _ any) error {
	f.calls = append(f.calls, commandStationID)
	if f.failOn[commandStationID] {
		return context.Canceled
	}
	return nil
}

func TestLeaseBrakeStopLeasedTarget_bestEffortOnMultiCS(t *testing.T) {
	t.Parallel()
	addr := uint16(12)
	vehicleID, err := domain.NewVehicleID()
	if err != nil {
		t.Fatal(err)
	}
	bus := &flakyDccBus{failOn: map[uint]bool{4: true}}
	brake := NewLeaseBrake(LeaseBrakeConfig{
		DccBus: bus,
		Roster: stubDriveTargetRoster{
			vehicles: []RosterVehicleEntry{{
				Vehicle: domain.Vehicle{ID: vehicleID, DCCAddress: ptrUint16(addr)},
			}},
		},
		Layouts: stubLeaseBrakeLayouts{csIDs: []uint{3, 4}},
	})
	err = brake.StopLeasedTarget(context.Background(), 2, domain.TakeoverTargetVehicle, vehicleID.String())
	if err == nil {
		t.Fatal("expected error from failed publish")
	}
	if len(bus.calls) != 2 {
		t.Fatalf("publish calls = %d, want 2", len(bus.calls))
	}
}
