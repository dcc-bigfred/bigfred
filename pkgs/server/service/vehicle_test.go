package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/repo/migrations"
	"github.com/keskad/loco/pkgs/server/service"
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
		Users:           repo.NewUsers(r),
		Pool:            repo.NewDCCAddressRanges(r),
		Vehicles:        repo.NewVehicles(r),
		Trains:          repo.NewTrains(r),
		TrainMembers:    repo.NewTrainMembers(r),
		LayoutVehicles:  repo.NewLayoutVehicles(r),
		LayoutTrains:    repo.NewLayoutTrains(r),
	}
	cleanup := func() {
		_ = db.Close()
		_ = os.Remove(path)
	}
	return bundle, cleanup
}

func insertUser(t *testing.T, ctx context.Context, users *repo.Users, login string, role domain.Role) domain.User {
	t.Helper()
	now := time.Now().UTC()
	u := domain.User{Login: login, PINHash: "x", Role: role, CreatedAt: now, UpdatedAt: now}
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

	pool := service.NewDCCPoolService(bundle.Pool)
	svc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)

	v, err := svc.Create(ctx, service.VehicleCreateInput{
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
}

func TestVehicleCreateRejectsOutsidePool(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := service.NewDCCPoolService(bundle.Pool)
	if _, err := pool.Replace(ctx, user.ID, []service.PoolRange{{From: 100, To: 199}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	svc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)

	addrInside := uint16(150)
	if _, err := svc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Loco inside",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addrInside,
	}); err != nil {
		t.Fatalf("expected inside-pool create to succeed, got %v", err)
	}

	addrOutside := uint16(7000)
	_, err := svc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: user.ID,
		Name:        "Loco outside",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addrOutside,
	})
	if !errors.Is(err, service.ErrDCCAddressOutsidePool) {
		t.Fatalf("expected ErrDCCAddressOutsidePool, got %v", err)
	}
}

func TestVehicleCreateRejectsDuplicateDCC(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := service.NewDCCPoolService(bundle.Pool)
	if _, err := pool.Replace(ctx, user.ID, []service.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	svc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)

	addr := uint16(33)
	if _, err := svc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "A", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "B", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if !errors.Is(err, service.ErrDCCAddressTaken) {
		t.Fatalf("expected ErrDCCAddressTaken, got %v", err)
	}
}

func TestVehicleDeleteRefusedWhenInTrain(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	user := insertUser(t, ctx, bundle.Users, "driver", domain.RoleDriver)

	pool := service.NewDCCPoolService(bundle.Pool)
	if _, err := pool.Replace(ctx, user.ID, []service.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	vSvc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)
	tSvc := service.NewTrainService(bundle.Trains, bundle.TrainMembers, bundle.Vehicles)

	addr := uint16(7)
	v, err := vSvc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: user.ID, Name: "Loco", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	if _, err := tSvc.Create(ctx, service.TrainCreateInput{
		OwnerUserID: user.ID,
		Name:        "Test",
		Members:     []service.TrainMemberInput{{VehicleID: v.ID}},
	}); err != nil {
		t.Fatalf("create train: %v", err)
	}

	if _, err := vSvc.Delete(ctx, user.ID, v.ID); !errors.Is(err, service.ErrVehicleInUse) {
		t.Fatalf("expected ErrVehicleInUse, got %v", err)
	}
}
