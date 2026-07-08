package remotes

import (
	"testing"
)

func TestVirtualLocoStoreDefaults(t *testing.T) {
	t.Parallel()
	s := NewVirtualLocoStore()
	snap := s.Snapshot("z21:10.0.0.1:40001", 31)
	if snap.Address != 31 || snap.Speed != 0 || !snap.Forward {
		t.Fatalf("default snap = %+v, want addr=31 speed=0 forward=true", snap)
	}
}

func TestVirtualLocoStoreSetSpeed(t *testing.T) {
	t.Parallel()
	s := NewVirtualLocoStore()
	key := "z21:10.0.0.1:40001"
	snap := s.SetSpeed(key, 31, 42, false)
	if snap.Speed != 42 || snap.Forward {
		t.Fatalf("SetSpeed snap = %+v", snap)
	}
	got := s.Snapshot(key, 31)
	if got.Speed != 42 || got.Forward {
		t.Fatalf("Snapshot after SetSpeed = %+v", got)
	}
}

func TestVirtualLocoStoreSetFunction(t *testing.T) {
	t.Parallel()
	s := NewVirtualLocoStore()
	key := "withrottle:device1"
	snap := s.SetFunction(key, 10239, 4, true)
	if len(snap.Functions) <= 4 || !snap.Functions[4] {
		t.Fatalf("SetFunction snap = %+v", snap)
	}
	s.SetFunction(key, 10239, 4, false)
	got := s.Snapshot(key, 10239)
	if got.Functions[4] {
		t.Fatal("function 4 should be off after toggle")
	}
}

func TestVirtualLocoStorePerClientAddrIsolation(t *testing.T) {
	t.Parallel()
	s := NewVirtualLocoStore()
	s.SetSpeed("client-a", 10, 5, true)
	s.SetSpeed("client-b", 10, 20, true)
	s.SetSpeed("client-a", 11, 30, false)

	if snap := s.Snapshot("client-a", 10); snap.Speed != 5 {
		t.Fatalf("client-a addr 10 speed = %d", snap.Speed)
	}
	if snap := s.Snapshot("client-b", 10); snap.Speed != 20 {
		t.Fatalf("client-b addr 10 speed = %d", snap.Speed)
	}
	if snap := s.Snapshot("client-a", 11); snap.Speed != 30 || snap.Forward {
		t.Fatalf("client-a addr 11 = %+v", snap)
	}
}

func TestVirtualLocoStoreRemoveClient(t *testing.T) {
	t.Parallel()
	s := NewVirtualLocoStore()
	key := "z21:10.0.0.1:40001"
	s.SetSpeed(key, 31, 10, true)
	if !s.HasClient(key) {
		t.Fatal("expected client entry before RemoveClient")
	}
	s.RemoveClient(key)
	if s.HasClient(key) {
		t.Fatal("expected client entry gone after RemoveClient")
	}
	snap := s.Snapshot(key, 31)
	if snap.Speed != 0 {
		t.Fatalf("after RemoveClient defaults should apply, got %+v", snap)
	}
}

func TestVirtualLocoStoreToggleFunction(t *testing.T) {
	t.Parallel()
	s := NewVirtualLocoStore()
	key := "z21:10.0.0.1:40001"
	if snap := s.ToggleFunction(key, 31, 1); !snap.Functions[1] {
		t.Fatal("first toggle should turn F1 on")
	}
	if snap := s.ToggleFunction(key, 31, 1); snap.Functions[1] {
		t.Fatal("second toggle should turn F1 off")
	}
}
