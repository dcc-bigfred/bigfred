package domain

import "testing"

func TestNewVehicleIDFormat(t *testing.T) {
	id, err := NewVehicleID()
	if err != nil {
		t.Fatalf("NewVehicleID: %v", err)
	}
	if !id.Valid() {
		t.Fatalf("expected valid VehicleID, got %q", id)
	}
	if len(id) != len(vehicleIDPrefix)+idSuffixLen {
		t.Fatalf("expected length %d, got %d (%q)", len(vehicleIDPrefix)+idSuffixLen, len(id), id)
	}
}

func TestNewTrainIDFormat(t *testing.T) {
	id, err := NewTrainID()
	if err != nil {
		t.Fatalf("NewTrainID: %v", err)
	}
	if !id.Valid() {
		t.Fatalf("expected valid TrainID, got %q", id)
	}
}

func TestParseVehicleIDRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"", "T-abc", "V", "1", "vehicle-1"} {
		if _, ok := ParseVehicleID(raw); ok {
			t.Fatalf("expected invalid for %q", raw)
		}
	}
	if _, ok := ParseVehicleID("V-1"); !ok {
		t.Fatal("expected legacy V-1 to parse")
	}
}
