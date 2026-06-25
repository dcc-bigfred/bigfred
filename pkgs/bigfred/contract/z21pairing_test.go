package contract

import (
	"math/rand"
	"testing"
)

func TestValidPairingCV(t *testing.T) {
	valid := []int{111, 122, 155, 215, 255}
	for _, v := range valid {
		if !ValidPairingCV(v) {
			t.Fatalf("expected %d valid", v)
		}
	}
	invalid := []int{99, 100, 160, 266, 312, 110}
	for _, v := range invalid {
		if ValidPairingCV(v) {
			t.Fatalf("expected %d invalid", v)
		}
	}
}

func TestAllValidPairingCVsCount(t *testing.T) {
	vals := AllValidPairingCVs()
	if len(vals) != 50 {
		t.Fatalf("expected 50 values, got %d", len(vals))
	}
}

func TestZ21PairingWireRoundTrip(t *testing.T) {
	req := Z21PairingReqWire{
		LayoutID:         1,
		CommandStationID: 2,
		UserID:           3,
		PairingCV3:       122,
		PairingCV4:       145,
		DisplayLabel:     "122-145",
		VehicleIDs:       []string{"V-1"},
		AllowedAddrs:     []uint16{3},
		AllowAllVehicles: false,
		CreatedAt:        1,
	}
	raw, err := MarshalZ21PairingReq(req)
	if err != nil {
		t.Fatal(err)
	}
	out, err := UnmarshalZ21PairingReq(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.PairingCV3 != 122 || out.PairingCV4 != 145 {
		t.Fatalf("req round-trip: %+v", out)
	}

	active := Z21PairingActiveWire{
		UserID:           3,
		VehicleIDs:       []string{"V-1"},
		AllowedAddrs:     []uint16{3},
		AllowAllVehicles: false,
		PairedAt:         2,
		PairingCV3:       122,
		PairingCV4:       145,
		LastSeenAt:       3,
		ClientKey:        "10.0.0.5:54321",
	}
	raw, err = MarshalZ21PairingActive(active)
	if err != nil {
		t.Fatal(err)
	}
	outActive, err := UnmarshalZ21PairingActive(raw)
	if err != nil {
		t.Fatal(err)
	}
	if outActive.ClientKey != "10.0.0.5:54321" {
		t.Fatalf("active round-trip: %+v", outActive)
	}
}

func TestRandomPairingCV(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		if !ValidPairingCV(RandomPairingCV(rng)) {
			t.Fatalf("random value invalid on iteration %d", i)
		}
	}
}
