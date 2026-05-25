package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/service"
)

var testUserPool = []service.PoolRange{{From: 1000, To: 1999}}

var testAdminEff = domain.NewEffectiveRoles(domain.RoleAdmin)

func userSvc(bundle repo.UsersBundle) *service.UserService {
	pool := service.NewDCCPoolService(bundle.Pool)
	return service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains, pool)
}

func TestUserCreateAcceptsAdminAndDriver(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := userSvc(bundle)

	for name, role := range map[string]domain.Role{
		"driver": domain.RoleDriver,
		"admin":  domain.RoleAdmin,
	} {
		t.Run(name, func(t *testing.T) {
			pool := testUserPool
			if name == "admin" {
				pool = []service.PoolRange{{From: 2000, To: 2999}}
			}
			row, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
				Login:   name,
				PIN:     "123456",
				Role:    role,
				DCCPool: pool,
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
	svc := userSvc(bundle)

	_, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login:   "sm",
		PIN:     "123456",
		Role:    domain.RoleSignalman,
		DCCPool: testUserPool,
	})
	if !errors.Is(err, service.ErrUserRoleInvalid) {
		t.Fatalf("expected ErrUserRoleInvalid, got %v", err)
	}
}

func TestUserCreateRejectsShortPIN(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := userSvc(bundle)

	_, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login:   "alice",
		PIN:     "12",
		Role:    domain.RoleDriver,
		DCCPool: testUserPool,
	})
	if !errors.Is(err, service.ErrUserPINInvalid) {
		t.Fatalf("expected ErrUserPINInvalid, got %v", err)
	}
}

func TestUserCreateRejectsDuplicateLogin(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := userSvc(bundle)

	if _, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver, DCCPool: testUserPool,
	}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login: "alice", PIN: "999999", Role: domain.RoleAdmin, DCCPool: []service.PoolRange{{From: 2000, To: 2999}},
	})
	if !errors.Is(err, service.ErrUserLoginTaken) {
		t.Fatalf("expected ErrUserLoginTaken, got %v", err)
	}
}

func TestUserDeleteRefusedWhenOwnsVehicles(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := userSvc(bundle)
	pool := service.NewDCCPoolService(bundle.Pool)
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)

	created, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 1, To: 100}},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	vSvc := service.NewVehicleService(bundle.Vehicles, pool, bundle.TrainMembers)
	addr := uint16(42)
	if _, err := vSvc.Create(ctx, service.VehicleCreateInput{
		OwnerUserID: created.ID, Name: "Loco", Kind: domain.VehicleKindLoco, DCCAddress: &addr,
	}); err != nil {
		t.Fatalf("create vehicle: %v", err)
	}

	if err := svc.Delete(ctx, testAdminEff, admin.ID, created.ID); !errors.Is(err, service.ErrUserHasVehicles) {
		t.Fatalf("expected ErrUserHasVehicles, got %v", err)
	}
}

func TestUserDeleteRefusedWhenOwnsTrains(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := userSvc(bundle)
	pool := service.NewDCCPoolService(bundle.Pool)
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)

	created, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 1, To: 100}},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
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

	if err := svc.Delete(ctx, testAdminEff, admin.ID, created.ID); !errors.Is(err, service.ErrUserHasTrains) {
		t.Fatalf("expected ErrUserHasTrains, got %v", err)
	}
}

func TestUserSetActiveToggles(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := userSvc(bundle)
	admin := insertUser(t, ctx, bundle.Users, "admin", domain.RoleAdmin)

	row, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver, DCCPool: testUserPool,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !row.Active {
		t.Fatalf("default user must be active")
	}
	off, err := svc.SetActive(ctx, testAdminEff, admin.ID, row.ID, false)
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if off.Active {
		t.Fatalf("expected Active=false after deactivate")
	}
	on, err := svc.SetActive(ctx, testAdminEff, admin.ID, row.ID, true)
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
	svc := userSvc(bundle)

	row, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver, DCCPool: testUserPool,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	originalHash := row.PINHash

	adminRole := domain.RoleAdmin
	newPIN := "987654"
	updated, err := svc.Update(ctx, testAdminEff, row.ID, service.UserUpdateInput{
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

func TestUserUpdateChangesDCCPool(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := userSvc(bundle)

	row, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 100, To: 199}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newPool := []service.PoolRange{{From: 500, To: 599}}
	if _, err := svc.Update(ctx, testAdminEff, row.ID, service.UserUpdateInput{DCCPool: &newPool}); err != nil {
		t.Fatalf("update pool: %v", err)
	}
	pool, err := svc.GetDCCPool(ctx, row.ID)
	if err != nil {
		t.Fatalf("get pool: %v", err)
	}
	if len(pool) != 1 || pool[0].FromAddr != 500 || pool[0].ToAddr != 599 {
		t.Fatalf("unexpected pool: %+v", pool)
	}
}

func TestUserManageAdminOnly(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	svc := userSvc(bundle)
	driverEff := domain.NewEffectiveRoles(domain.RoleDriver)

	if _, err := svc.ListWithDCCPools(ctx, driverEff); !errors.Is(err, service.ErrUserForbidden) {
		t.Fatalf("expected ErrUserForbidden on list, got %v", err)
	}
	if _, err := svc.Create(ctx, driverEff, service.UserCreateInput{
		Login: "bob", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: testUserPool,
	}); !errors.Is(err, service.ErrUserForbidden) {
		t.Fatalf("expected ErrUserForbidden on create, got %v", err)
	}

	row, err := svc.Create(ctx, testAdminEff, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: testUserPool,
	})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}

	newLogin := "alice2"
	if _, err := svc.Update(ctx, driverEff, row.ID, service.UserUpdateInput{Login: &newLogin}); !errors.Is(err, service.ErrUserForbidden) {
		t.Fatalf("expected ErrUserForbidden on update, got %v", err)
	}
	if _, err := svc.SetActive(ctx, driverEff, row.ID, row.ID, false); !errors.Is(err, service.ErrUserForbidden) {
		t.Fatalf("expected ErrUserForbidden on deactivate, got %v", err)
	}
	if err := svc.Delete(ctx, driverEff, row.ID, row.ID); !errors.Is(err, service.ErrUserForbidden) {
		t.Fatalf("expected ErrUserForbidden on delete, got %v", err)
	}
}
