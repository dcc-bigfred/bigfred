package service

import "testing"

func TestMergeLayoutIDs(t *testing.T) {
	got := mergeLayoutIDs([]uint{2, 3}, 3, 0, 1)
	if len(got) != 3 || got[0] != 2 || got[1] != 3 || got[2] != 1 {
		t.Fatalf("mergeLayoutIDs: got %v", got)
	}
	if len(mergeLayoutIDs(nil, 5)) != 1 {
		t.Fatalf("expected single ensured layout")
	}
}

func TestCommandStationIDsFingerprint(t *testing.T) {
	if commandStationIDsFingerprint([]uint{3, 1, 2}) != "1,2,3" {
		t.Fatalf("expected sorted fingerprint")
	}
	if commandStationIDsFingerprint([]uint{5}) != "5" {
		t.Fatalf("expected single id")
	}
	if commandStationIDsFingerprint(nil) != "" {
		t.Fatalf("expected empty fingerprint for nil slice")
	}
}
