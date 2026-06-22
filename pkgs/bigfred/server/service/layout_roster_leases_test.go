package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

type captureRosterPublisher struct {
	allowed contract.AllowedVehicles
}

func (c *captureRosterPublisher) PublishLayoutAllowedVehicles(_ context.Context, snap contract.AllowedVehicles) error {
	c.allowed = snap
	return nil
}

func (c *captureRosterPublisher) PublishLayoutDefinedTrains(context.Context, contract.DefinedTrains) error {
	return nil
}

func TestAllowedVehiclesSnapshotFoldsVehicleLease(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()
	ctx := context.Background()

	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	lessee := insertUser(t, ctx, bundle.Users, "lessee", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, owner.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	vehicleSvc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.Users)
	addr := uint16(42)
	vehicle, err := vehicleSvc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "ET22",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	layoutSvc := freshLayoutSvc(t, ctx, bundle)
	cs := insertCommandStation(t, ctx, bundle.CommandStations, "Test CS")
	layout, err := layoutSvc.Create(ctx, testAdminEff, cmd.LayoutCreateInput{
		Name:              "Test layout",
		CreatedBy:         owner.ID,
		AdminPIN:          "1234",
		CommandStationIDs: []uint{cs.ID},
	})
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}

	rosterSvc := service.NewLayoutVehicleService(
		bundle.LayoutVehicles,
		bundle.LayoutTrains,
		bundle.Vehicles,
		bundle.Trains,
		bundle.TrainMembers,
		bundle.VehicleLeases,
		bundle.TrainLeases,
		bundle.Users,
		ws.NewHub(),
	)
	capture := &captureRosterPublisher{}
	rosterSvc.SetRedisRosterPublisher(capture)

	if _, err := rosterSvc.AddVehicle(ctx, layout.ID, owner.ID, vehicle.ID, domain.NewEffectiveRoles(domain.RoleDriver)); err != nil {
		t.Fatalf("add vehicle to roster: %v", err)
	}

	now := time.Now().UTC()
	ok, err := bundle.VehicleLeases.Create(ctx, &domain.VehicleLease{
		VehicleID:  vehicle.ID,
		FromUserID: owner.ID,
		ToUserID:   lessee.ID,
		StartedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
	}, false)
	if err != nil || !ok {
		t.Fatalf("create lease: ok=%v err=%v", ok, err)
	}

	if err := rosterSvc.SyncLayoutRosterToRedis(ctx, layout.ID); err != nil {
		t.Fatalf("sync roster: %v", err)
	}
	if len(capture.allowed.Vehicles) != 1 {
		t.Fatalf("expected one allowed vehicle, got %d", len(capture.allowed.Vehicles))
	}
	// While the lease is active the owner loses drive authority: only the
	// lessee may drive (and be an emergency-stop target for) the vehicle.
	ids := capture.allowed.Vehicles[0].ControllerUserIDs
	if len(ids) != 1 || ids[0] != lessee.ID {
		t.Fatalf("controllerUserIds = %v, want [%d]", ids, lessee.ID)
	}
}
