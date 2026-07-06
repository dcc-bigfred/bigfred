package slotlease

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDiagnosticSnapshot_emptyHoldersJSONIsArray(t *testing.T) {
	l := newTestLeaser(&fakeStation{}, 8, 80)
	if _, err := l.Select(1, "sess", "ws", 3); err != nil {
		t.Fatalf("Select: %v", err)
	}
	l.DeselectDeferred(1, "sess", 3)

	snap := l.DiagnosticSnapshot()
	if len(snap.Leases) != 1 {
		t.Fatalf("len(Leases) = %d, want 1 grace lease", len(snap.Leases))
	}
	if len(snap.Leases[0].Holders) != 0 {
		t.Fatalf("Holders = %#v, want empty slice", snap.Leases[0].Holders)
	}
	b, err := json.Marshal(snap.Leases[0])
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) == "" || string(b[1:]) == "" {
		t.Fatal("empty marshal result")
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	holders, ok := raw["holders"].([]any)
	if !ok {
		t.Fatalf("holders JSON = %T %#v, want []", raw["holders"], raw["holders"])
	}
	if len(holders) != 0 {
		t.Fatalf("holders len = %d, want 0", len(holders))
	}
}

func TestDiagnosticSnapshot_reflectsLeasesAndLimits(t *testing.T) {
	st := &fakeStation{}
	l := newTestLeaser(st, 8, 80)

	if _, err := l.Select(1, "sess-a", "ws", 3); err != nil {
		t.Fatalf("Select: %v", err)
	}
	if _, err := l.Select(2, "sess-b", "z21", 5); err != nil {
		t.Fatalf("Select: %v", err)
	}

	snap := l.DiagnosticSnapshot()
	if snap.MaxPerUser != 8 {
		t.Fatalf("MaxPerUser = %d, want 8", snap.MaxPerUser)
	}
	if snap.MaxSlots != 80 {
		t.Fatalf("MaxSlots = %d, want 80", snap.MaxSlots)
	}
	if snap.Used != 2 {
		t.Fatalf("Used = %d, want 2", snap.Used)
	}
	if snap.PerUser[1] != 1 || snap.PerUser[2] != 1 {
		t.Fatalf("PerUser = %#v, want one lease each for users 1 and 2", snap.PerUser)
	}
	if len(snap.Leases) != 2 {
		t.Fatalf("len(Leases) = %d, want 2", len(snap.Leases))
	}
	found := map[uint16]LeaseInfo{}
	for _, le := range snap.Leases {
		found[le.Addr] = le
	}
	if le, ok := found[3]; !ok || le.Kind != "single" || len(le.Holders) != 1 || le.Holders[0].Source != "ws" {
		t.Fatalf("addr 3 lease = %#v", found[3])
	}
	if le, ok := found[5]; !ok || le.Holders[0].Source != "z21" {
		t.Fatalf("addr 5 lease = %#v", found[5])
	}
	if snap.At <= 0 {
		t.Fatal("At timestamp missing")
	}
}

func TestDiagEvents_firesOnMutation(t *testing.T) {
	l := newTestLeaser(&fakeStation{}, 8, 80)
	events := l.DiagEvents()

	if _, err := l.Select(1, "s", "ws", 10); err != nil {
		t.Fatalf("Select: %v", err)
	}
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("expected diag event after Select")
	}
}
