package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

func TestDCCPoolReplaceRejectsEmptyPool(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := service.NewDCCPoolService(bundle.Pool)
	userSvc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains, pool)

	row, err := userSvc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 1, To: 10}},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, err = pool.Replace(ctx, row.ID, nil)
	if !errors.Is(err, service.ErrDCCPoolEmpty) {
		t.Fatalf("expected ErrDCCPoolEmpty, got %v", err)
	}
}

func TestDCCPoolReplaceRejectsOutOfBounds(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := service.NewDCCPoolService(bundle.Pool)
	userSvc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains, pool)

	row, err := userSvc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 1, To: 10}},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	cases := []service.PoolRange{
		{From: 0, To: 10},
		{From: 1, To: 10000},
		{From: 100, To: 50},
	}
	for _, tc := range cases {
		_, err := pool.Replace(ctx, row.ID, []service.PoolRange{tc})
		if !errors.Is(err, service.ErrDCCPoolRangeInvalid) {
			t.Fatalf("range %+v: expected ErrDCCPoolRangeInvalid, got %v", tc, err)
		}
	}
}

func TestDCCPoolReplaceRejectsOverlapWithOtherUser(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := service.NewDCCPoolService(bundle.Pool)
	userSvc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains, pool)

	alice, err := userSvc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 100, To: 199}},
	})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := userSvc.Create(ctx, service.UserCreateInput{
		Login: "bob", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 300, To: 399}},
	})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	_, err = pool.Replace(ctx, bob.ID, []service.PoolRange{{From: 150, To: 250}})
	if !errors.Is(err, service.ErrDCCPoolOverlap) {
		t.Fatalf("expected ErrDCCPoolOverlap, got %v", err)
	}

	// Same user may replace without false overlap against own old rows.
	if _, err := pool.Replace(ctx, alice.ID, []service.PoolRange{{From: 100, To: 199}}); err != nil {
		t.Fatalf("replace own pool: %v", err)
	}
}

func TestUserCreateRequiresDCCPool(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := service.NewDCCPoolService(bundle.Pool)
	userSvc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains, pool)

	_, err := userSvc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
	})
	if !errors.Is(err, service.ErrDCCPoolEmpty) {
		t.Fatalf("expected ErrDCCPoolEmpty, got %v", err)
	}
}

func TestUserCreateDoesNotPersistWhenPoolOverlaps(t *testing.T) {
	bundle, cleanup := freshRepo(t)
	defer cleanup()

	ctx := context.Background()
	pool := service.NewDCCPoolService(bundle.Pool)
	userSvc := service.NewUserService(bundle.Users, bundle.Vehicles, bundle.Trains, pool)

	if _, err := userSvc.Create(ctx, service.UserCreateInput{
		Login: "alice", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 100, To: 199}},
	}); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	_, err := userSvc.Create(ctx, service.UserCreateInput{
		Login: "bob", PIN: "123456", Role: domain.RoleDriver,
		DCCPool: []service.PoolRange{{From: 150, To: 250}},
	})
	if !errors.Is(err, service.ErrDCCPoolOverlap) {
		t.Fatalf("expected ErrDCCPoolOverlap, got %v", err)
	}

	n, err := bundle.Users.Count(ctx)
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 user after failed create, got %d", n)
	}
}
