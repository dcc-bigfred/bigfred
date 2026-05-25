package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

func TestTrainAdminCanMutateOthersTrain(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)

	pool := service.NewDCCPoolService(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, owner.ID, []service.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed owner pool: %v", err)
	}
	vSvc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)
	tSvc := service.NewTrainService(bundle.Trains, bundle.TrainMembers, bundle.Vehicles)

	addr := uint16(42)
	v, err := vSvc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	train, err := tSvc.Create(ctx, service.TrainCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Freight",
		Members:     []service.TrainMemberInput{{VehicleID: v.ID}},
	})
	if err != nil {
		t.Fatalf("create train: %v", err)
	}

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	newName := "Renamed by admin"
	updated, err := tSvc.Update(ctx, admin.ID, train.Train.ID, adminEff, service.TrainUpdateInput{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("admin update: %v", err)
	}
	if updated.Train.Name != newName {
		t.Fatalf("name = %q, want %q", updated.Train.Name, newName)
	}

	if _, err := tSvc.Delete(ctx, admin.ID, train.Train.ID, adminEff); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}

func TestTrainNonOwnerDriverCannotMutateOthersTrain(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	other := insertUser(t, ctx, bundle.Users, "other", domain.RoleDriver)

	pool := service.NewDCCPoolService(bundle.Pool)
	vSvc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)
	tSvc := service.NewTrainService(bundle.Trains, bundle.TrainMembers, bundle.Vehicles)

	v, err := vSvc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	train, err := tSvc.Create(ctx, service.TrainCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Freight",
		Members:     []service.TrainMemberInput{{VehicleID: v.ID}},
	})
	if err != nil {
		t.Fatalf("create train: %v", err)
	}

	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)
	newName := "Hijacked"
	_, err = tSvc.Update(ctx, other.ID, train.Train.ID, driverEff, service.TrainUpdateInput{
		Name: &newName,
	})
	if !errors.Is(err, service.ErrTrainNotOwned) {
		t.Fatalf("expected ErrTrainNotOwned on update, got %v", err)
	}
	_, err = tSvc.Delete(ctx, other.ID, train.Train.ID, driverEff)
	if !errors.Is(err, service.ErrTrainNotOwned) {
		t.Fatalf("expected ErrTrainNotOwned on delete, got %v", err)
	}
}
