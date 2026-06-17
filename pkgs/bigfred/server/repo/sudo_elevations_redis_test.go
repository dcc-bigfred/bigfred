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

func TestRedisSudoElevationsGrantRenewExpire(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := repo.NewRedisSudoElevations(client)
	ctx := context.Background()
	now := time.Now().UTC()

	row := domain.SudoElevation{
		UserID:    7,
		LayoutID:  3,
		GrantedAt: now,
		ExpiresAt: now.Add(time.Minute),
	}
	if err := store.Upsert(ctx, &row); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	firstGrant := row.GrantedAt

	got, err := store.FindActive(ctx, 7, 3, now)
	if err != nil {
		t.Fatalf("find active: %v", err)
	}
	if !got.IsActive(now) {
		t.Fatal("expected active grant")
	}

	renewAt := now.Add(10 * time.Second)
	row.GrantedAt = renewAt
	row.ExpiresAt = renewAt.Add(time.Minute)
	if err := store.Upsert(ctx, &row); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if !row.GrantedAt.Equal(firstGrant) {
		t.Fatalf("renew should preserve grantedAt; got %v want %v", row.GrantedAt, firstGrant)
	}

	if err := store.Delete(ctx, 7, 3); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.FindActive(ctx, 7, 3, now); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestRedisSudoElevationsTTLExpiry(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := repo.NewRedisSudoElevations(client)
	ctx := context.Background()
	now := time.Now().UTC()

	row := domain.SudoElevation{
		UserID:    1,
		LayoutID:  1,
		GrantedAt: now,
		ExpiresAt: now.Add(50 * time.Millisecond),
	}
	if err := store.Upsert(ctx, &row); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	mr.FastForward(60 * time.Millisecond)

	if _, err := store.FindActive(ctx, 1, 1, time.Now().UTC()); err == nil {
		t.Fatal("expected grant to expire with key TTL")
	}
}

func TestRedisSudoElevationsRequiresJanitor(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	store := repo.NewRedisSudoElevations(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	if store.RequiresJanitor() {
		t.Fatal("redis sudo store should not need janitor")
	}
}
