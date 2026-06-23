package cmd_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

type testLeaseRoster struct {
	inner *cmd.LayoutRoster
}

func (t testLeaseRoster) ListVehicles(ctx context.Context, layoutID uint) ([]cmd.RosterVehicleEntry, error) {
	return t.inner.ListVehicles(ctx, layoutID)
}

func (t testLeaseRoster) ListTrains(ctx context.Context, layoutID uint) ([]cmd.RosterTrainEntry, error) {
	return t.inner.ListTrains(ctx, layoutID)
}

func (t testLeaseRoster) SyncLayoutRoster(context.Context, uint) error {
	return nil
}

func newLeaseSvc(bundle repo.UsersBundle, roster *cmd.LayoutRoster) *cmd.Lease {
	return cmd.NewLease(cmd.LeaseConfig{
		VehicleLeases:  bundle.VehicleLeases,
		TrainLeases:    bundle.TrainLeases,
		LayoutVehicles: bundle.LayoutVehicles,
		LayoutTrains:   bundle.LayoutTrains,
		Vehicles:       bundle.Vehicles,
		Trains:         bundle.Trains,
		Users:          bundle.Users,
		Roster:         testLeaseRoster{inner: roster},
	})
}

func setupLeasedVehicleFixture(t *testing.T) (
	ctx context.Context,
	bundle repo.UsersBundle,
	cleanup func(),
	owner domain.User,
	admin domain.User,
	lessee domain.User,
	layout domain.Layout,
	vehicle domain.Vehicle,
	leaseSvc *cmd.Lease,
) {
	t.Helper()
	bundle, cleanup = freshRepo(t)
	ctx = context.Background()

	owner = insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	admin = insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)
	lessee = insertUser(t, ctx, bundle.Users, "eve", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, owner.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	vehicleSvc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.Users)
	addr := uint16(42)
	var err error
	vehicle, err = vehicleSvc.Create(ctx, cmd.VehicleCreateInput{
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
	layout, err = layoutSvc.Create(ctx, testAdminEff, cmd.LayoutCreateInput{
		Name:              "Test layout",
		CreatedBy:         owner.ID,
		AdminPIN:          "1234",
		CommandStationIDs: []uint{cs.ID},
	})
	if err != nil {
		t.Fatalf("create layout: %v", err)
	}

	roster := cmd.NewLayoutRoster(
		bundle.LayoutVehicles,
		bundle.LayoutTrains,
		bundle.Vehicles,
		bundle.Trains,
		bundle.TrainMembers,
		bundle.Users,
		nil,
	)
	if _, err := roster.AddVehicle(ctx, layout.ID, owner.ID, vehicle.ID, domain.NewEffectiveRoles(domain.RoleDriver)); err != nil {
		t.Fatalf("add vehicle to roster: %v", err)
	}
	leaseSvc = newLeaseSvc(bundle, roster)
	return
}

func TestAdminCreatesLeaseForOtherOwner(t *testing.T) {
	ctx, _, cleanup, owner, admin, lessee, layout, vehicle, leaseSvc := setupLeasedVehicleFixture(t)
	defer cleanup()

	entry, err := leaseSvc.Create(
		ctx,
		layout.ID,
		admin,
		testAdminEff,
		domain.TakeoverTargetVehicle,
		vehicle.ID.String(),
		lessee.ID,
		80,
		30*time.Minute,
	)
	if err != nil {
		t.Fatalf("create lease: %v", err)
	}
	if entry.FromUserID != owner.ID {
		t.Fatalf("FromUserID = %d, want owner %d", entry.FromUserID, owner.ID)
	}
	if entry.ToUserID != lessee.ID {
		t.Fatalf("ToUserID = %d, want %d", entry.ToUserID, lessee.ID)
	}
	if entry.TargetName != "ET22" {
		t.Fatalf("TargetName = %q, want ET22", entry.TargetName)
	}
}

func TestNonOwnerCannotCreateLease(t *testing.T) {
	ctx, bundle, cleanup, owner, _, lessee, layout, vehicle, leaseSvc := setupLeasedVehicleFixture(t)
	defer cleanup()

	other := insertUser(t, ctx, bundle.Users, "other", domain.RoleDriver)
	_ = owner

	_, err := leaseSvc.Create(
		ctx,
		layout.ID,
		other,
		domain.NewEffectiveRoles(domain.RoleDriver),
		domain.TakeoverTargetVehicle,
		vehicle.ID.String(),
		lessee.ID,
		80,
		30*time.Minute,
	)
	if !errors.Is(err, svcerrors.ErrLeaseNotOwner) {
		t.Fatalf("expected ErrLeaseNotOwner, got %v", err)
	}
}

func TestAdminRevokesOtherOwnersLease(t *testing.T) {
	ctx, _, cleanup, owner, admin, lessee, layout, vehicle, leaseSvc := setupLeasedVehicleFixture(t)
	defer cleanup()

	if _, err := leaseSvc.Create(
		ctx,
		layout.ID,
		admin,
		testAdminEff,
		domain.TakeoverTargetVehicle,
		vehicle.ID.String(),
		lessee.ID,
		80,
		30*time.Minute,
	); err != nil {
		t.Fatalf("create lease: %v", err)
	}

	if err := leaseSvc.Revoke(
		ctx,
		layout.ID,
		admin,
		testAdminEff,
		domain.TakeoverTargetVehicle,
		vehicle.ID.String(),
	); err != nil {
		t.Fatalf("revoke lease: %v", err)
	}

	granted, err := leaseSvc.ListGranted(ctx, layout.ID, owner, domain.NewEffectiveRoles(domain.RoleDriver))
	if err != nil {
		t.Fatalf("list granted: %v", err)
	}
	if len(granted) != 0 {
		t.Fatalf("expected no granted leases after revoke, got %d", len(granted))
	}
}

func TestAdminListGrantedIncludesOthersLeases(t *testing.T) {
	ctx, _, cleanup, owner, admin, lessee, layout, vehicle, leaseSvc := setupLeasedVehicleFixture(t)
	defer cleanup()

	if _, err := leaseSvc.Create(
		ctx,
		layout.ID,
		admin,
		testAdminEff,
		domain.TakeoverTargetVehicle,
		vehicle.ID.String(),
		lessee.ID,
		80,
		30*time.Minute,
	); err != nil {
		t.Fatalf("create lease: %v", err)
	}

	granted, err := leaseSvc.ListGranted(ctx, layout.ID, admin, testAdminEff)
	if err != nil {
		t.Fatalf("list granted: %v", err)
	}
	if len(granted) != 1 {
		t.Fatalf("admin granted list len = %d, want 1", len(granted))
	}
	if granted[0].FromUserID != owner.ID {
		t.Fatalf("FromUserID = %d, want %d", granted[0].FromUserID, owner.ID)
	}

	ownerGranted, err := leaseSvc.ListGranted(ctx, layout.ID, owner, domain.NewEffectiveRoles(domain.RoleDriver))
	if err != nil {
		t.Fatalf("owner list granted: %v", err)
	}
	if len(ownerGranted) != 1 {
		t.Fatalf("owner granted list len = %d, want 1", len(ownerGranted))
	}
}
