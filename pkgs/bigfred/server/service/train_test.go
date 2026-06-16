package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
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


func TestUpdateMemberMultiplier(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)

	pool := service.NewDCCPoolService(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, owner.ID, []service.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed owner pool: %v", err)
	}
	vSvc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)
	tSvc := service.NewTrainService(bundle.Trains, bundle.TrainMembers, bundle.Vehicles)

	leadAddr := uint16(10)
	lead, err := vSvc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Lead",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &leadAddr,
	})
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	trailAddr := uint16(11)
	trail, err := vSvc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Trail",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &trailAddr,
	})
	if err != nil {
		t.Fatalf("create trail: %v", err)
	}

	train, err := tSvc.Create(ctx, service.TrainCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Consist",
		Members: []service.TrainMemberInput{
			{VehicleID: lead.ID},
			{VehicleID: trail.ID, SpeedMultiplier: 1.0},
		},
	})
	if err != nil {
		t.Fatalf("create train: %v", err)
	}
	trailMember := train.Members[1]
	ownerEff := domain.NewEffectiveRoles(domain.RoleDriver)

	updated, err := tSvc.UpdateMemberMultiplier(ctx, owner.ID, train.Train.ID, trailMember.ID, ownerEff, 1.25)
	if err != nil {
		t.Fatalf("update multiplier: %v", err)
	}
	if updated.SpeedMultiplier != 1.25 {
		t.Fatalf("multiplier = %v, want 1.25", updated.SpeedMultiplier)
	}

	leadMember := train.Members[0]
	if _, err := tSvc.UpdateMemberMultiplier(ctx, owner.ID, train.Train.ID, leadMember.ID, ownerEff, 1.5); !errors.Is(err, service.ErrTrainLeadingMultiplierImmutable) {
		t.Fatalf("leading update err = %v, want immutable", err)
	}
}
