package cmd_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
)

func TestTrainAdminCanMutateOthersTrain(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)

	pool := cmd.NewDCCPool(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, owner.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed owner pool: %v", err)
	}
	vSvc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.Users)
	tSvc := cmd.NewTrain(bundle.Trains, bundle.TrainMembers, bundle.Vehicles, bundle.LayoutTrains, bundle.Users)

	addr := uint16(42)
	v, err := vSvc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	train, err := tSvc.Create(ctx, cmd.TrainCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Freight",
		Members:     []cmd.TrainMemberInput{{VehicleID: v.ID}},
	})
	if err != nil {
		t.Fatalf("create train: %v", err)
	}

	adminEff := domain.NewEffectiveRoles(domain.RoleAdmin)
	newName := "Renamed by admin"
	updated, err := tSvc.Update(ctx, admin.ID, train.Train.ID, adminEff, cmd.TrainUpdateInput{
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

	pool := cmd.NewDCCPool(bundle.Pool)
	vSvc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.Users)
	tSvc := cmd.NewTrain(bundle.Trains, bundle.TrainMembers, bundle.Vehicles, bundle.LayoutTrains, bundle.Users)

	v, err := vSvc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Loco",
		Kind:        domain.VehicleKindLoco,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	train, err := tSvc.Create(ctx, cmd.TrainCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Freight",
		Members:     []cmd.TrainMemberInput{{VehicleID: v.ID}},
	})
	if err != nil {
		t.Fatalf("create train: %v", err)
	}

	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)
	newName := "Hijacked"
	_, err = tSvc.Update(ctx, other.ID, train.Train.ID, driverEff, cmd.TrainUpdateInput{
		Name: &newName,
	})
	if !errors.Is(err, svcerrors.ErrTrainNotOwned) {
		t.Fatalf("expected ErrTrainNotOwned on update, got %v", err)
	}
	_, err = tSvc.Delete(ctx, other.ID, train.Train.ID, driverEff)
	if !errors.Is(err, svcerrors.ErrTrainNotOwned) {
		t.Fatalf("expected ErrTrainNotOwned on delete, got %v", err)
	}
}

func TestUpdateMemberMultiplier(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	owner := insertUser(t, ctx, bundle.Users, "owner", domain.RoleDriver)

	pool := cmd.NewDCCPool(bundle.Pool)
	if _, err := pool.Replace(ctx, testAdminEff, owner.ID, []cmd.PoolRange{{From: 1, To: 9999}}); err != nil {
		t.Fatalf("seed owner pool: %v", err)
	}
	vSvc := cmd.NewVehicle(bundle.Vehicles, pool, bundle.TrainMembers, bundle.LayoutVehicles, bundle.Users)
	tSvc := cmd.NewTrain(bundle.Trains, bundle.TrainMembers, bundle.Vehicles, bundle.LayoutTrains, bundle.Users)

	leadAddr := uint16(10)
	lead, err := vSvc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Lead",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &leadAddr,
	})
	if err != nil {
		t.Fatalf("create lead: %v", err)
	}
	trailAddr := uint16(11)
	trail, err := vSvc.Create(ctx, cmd.VehicleCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Trail",
		Kind:        domain.VehicleKindLoco,
		DCCAddress:  &trailAddr,
	})
	if err != nil {
		t.Fatalf("create trail: %v", err)
	}

	train, err := tSvc.Create(ctx, cmd.TrainCreateInput{
		OwnerUserID: owner.ID,
		Name:        "Consist",
		Members: []cmd.TrainMemberInput{
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
	if _, err := tSvc.UpdateMemberMultiplier(ctx, owner.ID, train.Train.ID, leadMember.ID, ownerEff, 1.5); !errors.Is(err, svcerrors.ErrTrainLeadingMultiplierImmutable) {
		t.Fatalf("leading update err = %v, want immutable", err)
	}

	excluded := true
	updated, err = tSvc.UpdateMember(ctx, owner.ID, train.Train.ID, trailMember.ID, ownerEff, cmd.TrainMemberPatchInput{
		ExcludeFromSpeed: &excluded,
	})
	if err != nil {
		t.Fatalf("exclude trail: %v", err)
	}
	if !updated.ExcludeFromSpeed {
		t.Fatal("expected trail excluded from speed")
	}
	if _, err := tSvc.UpdateMember(ctx, owner.ID, train.Train.ID, leadMember.ID, ownerEff, cmd.TrainMemberPatchInput{
		ExcludeFromSpeed: &excluded,
	}); !errors.Is(err, svcerrors.ErrTrainLeadingSpeedControlImmutable) {
		t.Fatalf("leading exclude err = %v, want immutable", err)
	}

	delay := 150
	updated, err = tSvc.UpdateMember(ctx, owner.ID, train.Train.ID, trailMember.ID, ownerEff, cmd.TrainMemberPatchInput{
		StartDelayMs: &delay,
	})
	if err != nil {
		t.Fatalf("set start delay: %v", err)
	}
	if updated.StartDelayMs != 150 {
		t.Fatalf("startDelayMs = %d, want 150", updated.StartDelayMs)
	}

	rampMs := 2500
	rampSteps := 3
	updated, err = tSvc.UpdateMember(ctx, owner.ID, train.Train.ID, trailMember.ID, ownerEff, cmd.TrainMemberPatchInput{
		AccelRampMs:       &rampMs,
		AccelRampMaxSteps: &rampSteps,
	})
	if err != nil {
		t.Fatalf("set accel ramp on trail: %v", err)
	}
	if updated.AccelRampMs != 2500 || updated.AccelRampMaxSteps != 3 {
		t.Fatalf("accel ramp = %d/%d, want 2500/3", updated.AccelRampMs, updated.AccelRampMaxSteps)
	}

	leadDelay := 200
	updated, err = tSvc.UpdateMember(ctx, owner.ID, train.Train.ID, leadMember.ID, ownerEff, cmd.TrainMemberPatchInput{
		StartDelayMs: &leadDelay,
	})
	if err != nil {
		t.Fatalf("set start delay on leading: %v", err)
	}
	if updated.StartDelayMs != 200 {
		t.Fatalf("leading startDelayMs = %d, want 200", updated.StartDelayMs)
	}
	leadRampMs := 1000
	updated, err = tSvc.UpdateMember(ctx, owner.ID, train.Train.ID, leadMember.ID, ownerEff, cmd.TrainMemberPatchInput{
		AccelRampMs: &leadRampMs,
	})
	if err != nil {
		t.Fatalf("set accel ramp on leading: %v", err)
	}
	if updated.AccelRampMs != 1000 {
		t.Fatalf("leading accelRampMs = %d, want 1000", updated.AccelRampMs)
	}

	brakeMs := 1500
	brakeSteps := 2
	updated, err = tSvc.UpdateMember(ctx, owner.ID, train.Train.ID, trailMember.ID, ownerEff, cmd.TrainMemberPatchInput{
		BrakeRampMs:       &brakeMs,
		BrakeRampMaxSteps: &brakeSteps,
	})
	if err != nil {
		t.Fatalf("set brake ramp on trail: %v", err)
	}
	if updated.BrakeRampMs != 1500 || updated.BrakeRampMaxSteps != 2 {
		t.Fatalf("brake ramp = %d/%d, want 1500/2", updated.BrakeRampMs, updated.BrakeRampMaxSteps)
	}
}
