package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

func TestRedisVehicleLeasesInsertListRevoke(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	store := repo.NewRedisVehicleLeases(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	now := time.Now().UTC()

	row := domain.VehicleLease{
		VehicleID:  9,
		FromUserID: 1,
		ToUserID:   2,
		StartedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
	}
	if err := store.Insert(ctx, &row); err != nil {
		t.Fatalf("insert: %v", err)
	}
	active, err := store.ListActive(ctx, []uint{9}, now)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(active) != 1 || active[0].ToUserID != 2 {
		t.Fatalf("active = %+v", active)
	}
	if err := store.Revoke(ctx, 9, now); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	active, err = store.ListActive(ctx, []uint{9}, now)
	if err != nil {
		t.Fatalf("list after revoke: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active lease, got %+v", active)
	}
}

func TestRedisTakeoverRequestsPendingGrantDelete(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	store := repo.NewRedisTakeoverRequests(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	now := time.Now().UTC()

	row := domain.TakeoverRequest{
		LayoutID:        1,
		InterlockingID:  2,
		SignalmanUserID: 5,
		DriverUserID:    7,
		Target:          domain.TakeoverTargetVehicle,
		TargetID:        42,
		RequestedAt:     now,
		AutoGrantAt:     now.Add(15 * time.Second),
		State:           domain.TakeoverStatePending,
	}
	if err := store.Insert(ctx, &row); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if row.ID == 0 {
		t.Fatal("expected assigned id")
	}
	if _, err := store.FindPendingForTarget(ctx, domain.TakeoverTargetVehicle, 42); err != nil {
		t.Fatalf("find pending for target: %v", err)
	}
	decision := now
	row.State = domain.TakeoverStateGranted
	row.DecisionAt = &decision
	leaseID := row.TargetID
	row.GrantedLeaseID = &leaseID
	if err := store.Update(ctx, &row); err != nil {
		t.Fatalf("grant update: %v", err)
	}
	granted, err := store.ListGrantedBySignalman(ctx, 5)
	if err != nil || len(granted) != 1 {
		t.Fatalf("granted by signalman: %v len=%d", err, len(granted))
	}
	row.State = domain.TakeoverStateReleased
	released := now
	row.ReleasedAt = &released
	if err := store.Update(ctx, &row); err != nil {
		t.Fatalf("release update: %v", err)
	}
	if _, err := store.FindByID(ctx, row.ID); err == nil {
		t.Fatal("expected released row removed from redis")
	}
}
