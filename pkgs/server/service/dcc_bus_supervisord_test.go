package service

import "testing"

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
