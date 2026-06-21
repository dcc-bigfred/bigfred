package cmd_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/repo/migrations"
)

// freshRepo opens a brand-new SQLite db in a temp dir and runs every
// migration. Returns the wired *repo.* + cleanup.
func freshRepo(t *testing.T) (repo.UsersBundle, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "service_test.db")

	r, db, err := repo.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	migrations.MigrateUp(context.Background(), r)

	bundle := repo.UsersBundle{
		Users:                 repo.NewUsers(r),
		Pool:                  repo.NewDCCAddressRanges(r),
		Vehicles:              repo.NewVehicles(r),
		Trains:                repo.NewTrains(r),
		TrainMembers:          repo.NewTrainMembers(r),
		LayoutVehicles:        repo.NewLayoutVehicles(r),
		LayoutTrains:          repo.NewLayoutTrains(r),
		LayoutSignalmen:       repo.NewLayoutSignalmen(r),
		Layouts:               repo.NewLayouts(r),
		Interlockings:         repo.NewInterlockings(r),
		LayoutInterlockings:   repo.NewLayoutInterlockings(r),
		CommandStations:       repo.NewCommandStations(r),
		LayoutCommandStations: repo.NewLayoutCommandStations(r),
		SudoElevations:        repo.NewSudoElevations(r),
		VehicleLeases:         repo.NewVehicleLeases(r),
		TrainLeases:           repo.NewTrainLeases(r),
	}
	cleanup := func() {
		_ = db.Close()
		_ = os.Remove(path)
	}
	return bundle, cleanup
}

func insertCommandStation(t *testing.T, ctx context.Context, stations *repo.CommandStations, name string) domain.CommandStation {
	t.Helper()
	now := time.Now().UTC()
	cs := domain.CommandStation{
		Name:          name,
		Kind:          domain.CommandStationKindZ21,
		ConnectionURI: "udp://127.0.0.1:21105",
		SpeedSteps:    128,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := stations.Insert(ctx, &cs); err != nil {
		t.Fatalf("insert command station: %v", err)
	}
	return cs
}

func insertUser(t *testing.T, ctx context.Context, users *repo.Users, login string, role domain.Role) domain.User {
	t.Helper()
	now := time.Now().UTC()
	u := domain.User{Login: login, PINHash: "x", Role: role, Active: true, CreatedAt: now, UpdatedAt: now}
	if err := users.Insert(ctx, &u); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return u
}

func TestVehicleCreateAcceptsDummy(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Pool)
	svc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers)

	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Dummy boxcar",
		Kind:        domain.VehicleKindWagon,
		Number:      "92510",
	})
	if err != nil {
		t.Fatalf("create dummy: %v", err)
	}
	if v.DCCAddress != nil {
		t.Fatalf("expected dummy (nil DCC), got %v", v.DCCAddress)
	}
	if !v.IsDummy() {
		t.Fatalf("expected IsDummy true")
	}
	if !v.ID.Valid() {
		t.Fatalf("expected V- prefixed catalogue id, got %q", v.ID)
	}
}

func TestVehicleCreateRejectsOutsidePool(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, user.ID, []cmd.PoolRange{{From: 100, To: 199}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	svc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers)

	addrInside := uint16(150)
	if _, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Loco inside",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addrInside,
	}); err != nil {
		t.Fatalf("expected inside-pool create to succeed, got %v", err)
	}

	addrOutside := uint16(7000)
	_, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Loco outside",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addrOutside,
	})
	if !errors.Is(err, svcerrors.ErrDCCAddressOutsidePool) {
		t.Fatalf("expected ErrDCCAddressOutsidePool, got %v", err)
	}
}

func TestVehicleCreateRejectsDuplicateDCC(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, user.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	svc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers)

	addr := uint16(33)
	if _, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "A", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "B", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if !errors.Is(err, svcerrors.ErrDCCAddressTaken) {
		t.Fatalf("expected ErrDCCAddressTaken, got %v", err)
	}
}

func TestVehicleDeleteRefusedWhenInTrain(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, user.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	vSvc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers)
	tSvc := cmd.NewTrain(bundle.Trains, bundle.TrainMembers, bundle.Vehicles)

	addr := uint16(7)
	v, err := vSvc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "Loco", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	if _, err := tSvc.Create(ctx, cmd.TrainCreateInput{
		OwnerUserID: user.ID,
		Name:        "Test",
		Members:     []cmd.TrainMemberInput{{VehicleID: v.ID}},
	}); err != nil {
		t.Fatalf("create train: %v", err)
	}

	if _, err := vSvc.Delete(ctx, user.ID, v.ID, domain.NewEffectiveRoles(domain.RoleDriver)); !errors.Is(err, svcerrors.ErrVehicleInUse) {
		t.Fatalf("expected ErrVehicleInUse, got %v", err)
	}
}

func TestVehicleAdminCanMutateOthersVehicle(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)

	pool := cmd.NewDCCPool(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, owner.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed owner pool: %v", err)
	}
	svc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers)

	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	newName := "Renamed by admin"
	updated, err := svc.Update(ctx, admin.ID, v.ID, adminEff, cmd.VehicleUpdateInput{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("admin update: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("name = %q, want %q", updated.Name, newName)
	}

	if _, err := svc.Delete(ctx, admin.ID, v.ID, adminEff); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}

func TestVehicleNonOwnerDriverCannotMutateOthersVehicle(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	other := insertUser(t, ctx, bundle.Users, "other", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Pool)
	svc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers)

	v, err := svc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)
	newName := "Hijacked"
	_, err = svc.Update(ctx, other.ID, v.ID, driverEff, cmd.VehicleUpdateInput{Name: &newName})
	if !errors.Is(err, svcerrors.ErrVehicleNotOwned) {
		t.Fatalf("expected ErrVehicleNotOwned on update, got %v", err)
	}
	_, err = svc.Delete(ctx, other.ID, v.ID, driverEff)
	if !errors.Is(err, svcerrors.ErrVehicleNotOwned) {
		t.Fatalf("expected ErrVehicleNotOwned on delete, got %v", err)
	}
}
