package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

func TestUserCreateAcceptsAdminAndDriver(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains)

	for name, role := range map[string]domain.Role{
		"driver": domain.RoleDriver,
		"admin":  domain.RoleAdmin,
	} {
		t.Run(name, func(t *testing.T) {
			row, err := svc.Create(ctx, service.UserCreateInput{
				Login: name,
				PIN:   "123456",
				Role:  role,
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if row.Role != role {
				t.Fatalf("role mismatch: got %q want %q", row.Role, role)
			}
			if !row.Active {
				t.Fatalf("newly created user must be Active")
			}
		})
	}
}

func TestUserCreateRejectsSignalmanRole(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains)

	_, err := svc.Create(ctx, service.UserCreateInput{
		Login: "sm",
		PIN:   "123456",
		Role:  domain.RoleSignalman,
	})
	if !errors.Is(err, service.ErrUserRoleInvalid) {
		t.Fatalf("expected ErrUserRoleInvalid, got %v", err)
	}
}

func TestUserCreateRejectsShortPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains)

	_, err := svc.Create(ctx, service.UserCreateInput{
		Login: "alice",
		PIN:   "12",
		Role:  domain.RoleDriver,
	})
	if !errors.Is(err, service.ErrUserPINInvalid) {
		t.Fatalf("expected ErrUserPINInvalid, got %v", err)
	}
}

func TestUserCreateRejectsDuplicateLogin(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains)

	if _, err := svc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "999999", Role: domain.RoleAdmin,
	})
	if !errors.Is(err, service.ErrUserLoginTaken) {
		t.Fatalf("expected ErrUserLoginTaken, got %v", err)
	}
}

func TestUserDeleteRefusedWhenOwnsVehicles(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains)
	pool := service.NewDCCPoolService(bundle.Pool)

	created, err := svc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := pool.Replace(ctx, created.ID, []service.PoolRange{{From: 1, To: 100}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	vSvc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)
	addr := uint16(42)
	if _, err := vSvc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: created.ID, Name: "Loco", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	}); err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	if err := svc.Delete(ctx, created.ID); !errors.Is(err, service.ErrUserHasVehicles) {
		t.Fatalf("expected ErrUserHasVehicles, got %v", err)
	}
}

func TestUserDeleteRefusedWhenOwnsTrains(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains)
	pool := service.NewDCCPoolService(bundle.Pool)

	created, err := svc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := pool.Replace(ctx, created.ID, []service.PoolRange{{From: 1, To: 100}}); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	vSvc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)
	addr := uint16(42)
	v, err := vSvc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: created.ID, Name: "Loco", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	tSvc := service.NewTrainService(bundle.Trains, bundle.TrainMembers, bundle.Vehicles)
	if _, err := tSvc.Create(ctx, service.TrainCreateInput{
		OwnerUserID: created.ID,
		Name:        "Express",
		Members:     []service.TrainMemberInput{{VehicleID: v.ID}},
	}); err != nil {
		t.Fatalf("create train: %v", err)
	}

	// To isolate the train-specific branch we wipe the vehicle's
	// owner so the vehicles guard reports zero owned rows. The
	// train's owner stays put, so the next Delete call must hit
	// ErrUserHasTrains.
	v.OwnerUserID = 0
	if err := bundle.Vehicles.Update(ctx, &v); err != nil {
		t.Fatalf("reset vehicle owner: %v", err)
	}

	if err := svc.Delete(ctx, created.ID); !errors.Is(err, service.ErrUserHasTrains) {
		t.Fatalf("expected ErrUserHasTrains, got %v", err)
	}
}

func TestUserSetActiveToggles(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains)

	row, err := svc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !row.Active {
		t.Fatalf("default user must be active")
	}
	off, err := svc.SetActive(ctx, row.ID, false)
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if off.Active {
		t.Fatalf("expected Active=false after deactivate")
	}
	on, err := svc.SetActive(ctx, row.ID, true)
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if !on.Active {
		t.Fatalf("expected Active=true after reactivate")
	}
}

func TestUserUpdateChangesRoleAndPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains)

	row, err := svc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	originalHash := row.PINHash

	adminRole := domain.RoleAdmin
	newPIN := "987654"
	updated, err := svc.Update(ctx, row.ID, service.UserUpdateInput{
		Role: &adminRole,
		PIN:  &newPIN,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Role != domain.RoleAdmin {
		t.Fatalf("expected admin role after update")
	}
	if updated.PINHash == originalHash {
		t.Fatalf("PIN hash should change after rotation")
	}
}
