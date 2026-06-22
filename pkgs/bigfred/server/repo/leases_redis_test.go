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

func TestRedisVehicleLeasesCreateListRevoke(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	store := repo.NewRedisVehicleLeases(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	now := time.Now().UTC()
	vehicleID := domain.VehicleID("V-9")

	row := domain.VehicleLease{
		VehicleID:  vehicleID,
		FromUserID: 1,
		ToUserID:   2,
		StartedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
	}
	ok, err := store.Create(ctx, &row, false)
	if err != nil || !ok {
		t.Fatalf("create: ok=%v err=%v", ok, err)
	}
	active, err := store.ListActive(ctx, []domain.VehicleID{vehicleID}, now)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(active) != 1 || active[0].ToUserID != 2 {
		t.Fatalf("active = %+v", active)
	}
	byOwner, err := store.ListByOwner(ctx, 1)
	if err != nil || len(byOwner) != 1 {
		t.Fatalf("by owner: %v len=%d", err, len(byOwner))
	}
	byLessee, err := store.ListByLessee(ctx, 2)
	if err != nil || len(byLessee) != 1 {
		t.Fatalf("by lessee: %v len=%d", err, len(byLessee))
	}
	if err := store.Revoke(ctx, vehicleID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	active, err = store.ListActive(ctx, []domain.VehicleID{vehicleID}, now)
	if err != nil {
		t.Fatalf("list after revoke: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active lease, got %+v", active)
	}
	byOwner, err = store.ListByOwner(ctx, 1)
	if err != nil || len(byOwner) != 0 {
		t.Fatalf("by owner after revoke: %v len=%d", err, len(byOwner))
	}
}

func TestRedisVehicleLeasesCreateConflict(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	store := repo.NewRedisVehicleLeases(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	now := time.Now().UTC()
	vehicleID := domain.VehicleID("V-9")

	row := domain.VehicleLease{
		VehicleID:  vehicleID,
		FromUserID: 1,
		ToUserID:   2,
		StartedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
	}
	ok, err := store.Create(ctx, &row, false)
	if err != nil || !ok {
		t.Fatalf("first create: ok=%v err=%v", ok, err)
	}
	ok, err = store.Create(ctx, &row, false)
	if err != nil || ok {
		t.Fatalf("second create: ok=%v err=%v", ok, err)
	}
}

func TestRedisVehicleLeasesCreateOverwrite(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	store := repo.NewRedisVehicleLeases(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	now := time.Now().UTC()
	vehicleID := domain.VehicleID("V-9")

	manual := domain.VehicleLease{
		VehicleID:  vehicleID,
		FromUserID: 1,
		ToUserID:   2,
		StartedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
		Source:     "manual",
	}
	ok, err := store.Create(ctx, &manual, false)
	if err != nil || !ok {
		t.Fatalf("manual create: ok=%v err=%v", ok, err)
	}
	byLessee, err := store.ListByLessee(ctx, 2)
	if err != nil || len(byLessee) != 1 {
		t.Fatalf("lessee 2 before overwrite: %v len=%d", err, len(byLessee))
	}

	takeover := domain.VehicleLease{
		VehicleID:  vehicleID,
		FromUserID: 1,
		ToUserID:   3,
		StartedAt:  now,
		ExpiresAt:  now.Add(30 * time.Minute),
		Source:     "takeover",
	}
	ok, err = store.Create(ctx, &takeover, true)
	if err != nil || !ok {
		t.Fatalf("overwrite create: ok=%v err=%v", ok, err)
	}
	active, err := store.ListActive(ctx, []domain.VehicleID{vehicleID}, now)
	if err != nil || len(active) != 1 || active[0].ToUserID != 3 {
		t.Fatalf("active after overwrite = %+v err=%v", active, err)
	}
	byLessee, err = store.ListByLessee(ctx, 2)
	if err != nil || len(byLessee) != 0 {
		t.Fatalf("stale lessee 2 index: %v len=%d", err, len(byLessee))
	}
	byLessee, err = store.ListByLessee(ctx, 3)
	if err != nil || len(byLessee) != 1 {
		t.Fatalf("lessee 3 after overwrite: %v len=%d", err, len(byLessee))
	}
}

func TestRedisVehicleLeasesUpdate(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	store := repo.NewRedisVehicleLeases(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	now := time.Now().UTC()
	vehicleID := domain.VehicleID("V-9")

	row := domain.VehicleLease{
		VehicleID:  vehicleID,
		FromUserID: 1,
		ToUserID:   2,
		SpeedLimit: 50,
		StartedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
	}
	ok, err := store.Create(ctx, &row, false)
	if err != nil || !ok {
		t.Fatalf("create: ok=%v err=%v", ok, err)
	}
	row.SpeedLimit = 80
	row.ExpiresAt = now.Add(2 * time.Hour)
	if err := store.Update(ctx, &row); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, found, err := store.Get(ctx, vehicleID)
	if err != nil || !found || got.SpeedLimit != 80 {
		t.Fatalf("get after update: found=%v row=%+v err=%v", found, got, err)
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
		TargetID:        "V-42",
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
	if _, err := store.FindPendingForTarget(ctx, domain.TakeoverTargetVehicle, "V-42"); err != nil {
		t.Fatalf("find pending for target: %v", err)
	}
	decision := now
	row.State = domain.TakeoverStateGranted
	row.DecisionAt = &decision
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
