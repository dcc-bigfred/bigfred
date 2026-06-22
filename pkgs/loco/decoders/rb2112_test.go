package decoders

import "testing"

func TestRailboxRB2112SetGetBrightness(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewRailboxRB2112(cv)

	if err := d.SetBrightness(1, 50); err != nil {
		t.Fatalf("SetBrightness: %v", err)
	}
	if cv.values[41] != 128 {
		t.Fatalf("written CV41 = %d, want 128", cv.values[41])
	}

	got, err := d.GetBrightness(1)
	if err != nil {
		t.Fatalf("GetBrightness: %v", err)
	}
	if got != 50 {
		t.Fatalf("GetBrightness = %d, want 50", got)
	}
}

func TestRailboxRB2112BrightnessOutputRange(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewRailboxRB2112(cv)

	if err := d.SetBrightness(14, 50); err == nil {
		t.Fatal("expected error for out-of-range output 14")
	}

	if err := d.SetBrightness(9, 100); err != nil {
		t.Fatalf("SetBrightness output 9: %v", err)
	}
	if cv.values[106] != 255 {
		t.Fatalf("written CV106 = %d, want 255", cv.values[106])
	}

	if len(d.Outputs()) != int(railboxRB2112BrightnessOutputMax) {
		t.Fatalf("Outputs() len = %d, want %d", len(d.Outputs()), railboxRB2112BrightnessOutputMax)
	}
}

func TestRailboxRB2112SnapshotBrightness(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{41: 200, 106: 100}}
	d := NewRailboxRB2112(cv)

	states, err := d.SnapshotBrightness()
	if err != nil {
		t.Fatalf("SnapshotBrightness: %v", err)
	}
	if len(states) != int(railboxRB2112BrightnessOutputMax) {
		t.Fatalf("snapshot len = %d, want %d", len(states), railboxRB2112BrightnessOutputMax)
	}
	if states[0].CV != 41 || states[0].Value != 200 {
		t.Fatalf("output 1 state = %+v", states[0])
	}
	if states[8].CV != 106 || states[8].Value != 100 {
		t.Fatalf("output 9 state = %+v", states[8])
	}
}
