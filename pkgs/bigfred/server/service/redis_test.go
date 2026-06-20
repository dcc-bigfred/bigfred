package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisReachable_miniredis(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	if !RedisReachable(context.Background(), RedisServiceConfig{Addr: mr.Addr()}, time.Second) {
		t.Fatal("expected reachable miniredis")
	}
}

func TestRedisReachable_nothingListening(t *testing.T) {
	if RedisReachable(context.Background(), RedisServiceConfig{Addr: "127.0.0.1:1"}, 100*time.Millisecond) {
		t.Fatal("expected unreachable address")
	}
}

func TestResolveRedisManagement_explicitExternal(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	got := ResolveRedisManagement(context.Background(), RedisServiceConfig{Addr: mr.Addr()}, true, true)
	if got.Managed || got.Source != "explicit-external" {
		t.Fatalf("got %+v, want managed=false source=explicit-external", got)
	}
}

func TestResolveRedisManagement_autoDetected(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	got := ResolveRedisManagement(context.Background(), RedisServiceConfig{Addr: mr.Addr()}, false, true)
	if got.Managed || got.Source != "auto-detected" {
		t.Fatalf("got %+v, want managed=false source=auto-detected", got)
	}
}

func TestResolveRedisManagement_managedWhenAbsent(t *testing.T) {
	got := ResolveRedisManagement(context.Background(), RedisServiceConfig{Addr: "127.0.0.1:1"}, false, true)
	if !got.Managed || got.Source != "managed" {
		t.Fatalf("got %+v, want managed=true source=managed", got)
	}
}

func TestResolveRedisManagement_autoDetectDisabled(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	got := ResolveRedisManagement(context.Background(), RedisServiceConfig{Addr: mr.Addr()}, false, false)
	if !got.Managed || got.Source != "managed" {
		t.Fatalf("got %+v, want managed=true source=managed", got)
	}
}
