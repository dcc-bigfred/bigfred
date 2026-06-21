package decoders

import (
	"testing"
	"time"
)

func TestRailboxRB23xxSetGetBrightness(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewRailboxRB23xx(WithCVAccess(cv))

	if err := d.SetBrightness(1, 50); err != nil {
		t.Fatalf("SetBrightness: %v", err)
	}
	if cv.values[119] != 128 {
		t.Fatalf("written CV119 = %d, want 128", cv.values[119])
	}

	got, err := d.GetBrightness(1)
	if err != nil {
		t.Fatalf("GetBrightness: %v", err)
	}
	if got != 50 {
		t.Fatalf("GetBrightness = %d, want 50", got)
	}
}

func TestRailboxRB23xxBrightnessOutputRange(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewRailboxRB23xx(WithCVAccess(cv))

	if err := d.SetBrightness(12, 50); err == nil {
		t.Fatal("expected error for out-of-range output")
	}

	if len(d.Outputs()) != int(railboxRB23xxBrightnessOutputMax) {
		t.Fatalf("Outputs() len = %d, want %d", len(d.Outputs()), railboxRB23xxBrightnessOutputMax)
	}
}

func TestLokSoundv5SetGetBrightness(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewLokSoundv5(cv)

	if err := d.SetBrightness(4, 50); err != nil {
		t.Fatalf("SetBrightness: %v", err)
	}
	if cv.values[lokSoundV5IndexCV] != lokSoundV5IndexPageValue {
		t.Fatalf("CV31 = %d, want %d", cv.values[lokSoundV5IndexCV], lokSoundV5IndexPageValue)
	}
	if cv.values[lokSoundV5IndexPageCV] != lokSoundV5OutputConfigPage {
		t.Fatalf("CV32 = %d, want %d", cv.values[lokSoundV5IndexPageCV], lokSoundV5OutputConfigPage)
	}
	if cv.values[302] != 16 {
		t.Fatalf("written CV302 = %d, want 16", cv.values[302])
	}

	got, err := d.GetBrightness(4)
	if err != nil {
		t.Fatalf("GetBrightness: %v", err)
	}
	if got != 52 {
		t.Fatalf("GetBrightness = %d, want 52", got)
	}
}

func TestLokSoundv5BrightnessCVForOutput(t *testing.T) {
	d := NewLokSoundv5(&fakeCV{values: map[uint16]int{}})

	tests := []struct {
		output uint8
		wantCV uint16
	}{
		{1, 281},
		{2, 288},
		{4, 302},
		{18, 400},
	}

	for _, tt := range tests {
		cv, err := d.brightnessCVForOutput(tt.output)
		if err != nil {
			t.Fatalf("output %d: %v", tt.output, err)
		}
		if cv != tt.wantCV {
			t.Fatalf("output %d CV = %d, want %d", tt.output, cv, tt.wantCV)
		}
	}
}

func TestSetBrightnessRejectsOver100(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewRailboxRB23xx(WithCVAccess(cv))
	if err := d.SetBrightness(1, 101); err == nil {
		t.Fatal("expected error for percent > 100")
	}
}

func TestRailboxRB23xxSnapshotBrightness(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{119: 200, 120: 64}}
	d := NewRailboxRB23xx(WithCVAccess(cv))

	states, err := d.SnapshotBrightness()
	if err != nil {
		t.Fatalf("SnapshotBrightness: %v", err)
	}
	if len(states) != int(railboxRB23xxBrightnessOutputMax) {
		t.Fatalf("snapshot len = %d, want %d", len(states), railboxRB23xxBrightnessOutputMax)
	}
	if states[0].CV != 119 || states[0].Value != 200 {
		t.Fatalf("output 1 snapshot = %+v, want cv119=200", states[0])
	}
}

func TestRunBrightnessTestRestoresValues(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{119: 200, 120: 100}}
	d := NewRailboxRB23xx(WithCVAccess(cv))

	_, err := RunBrightnessTest(d, func(time.Duration) {})
	if err != nil {
		t.Fatalf("RunBrightnessTest: %v", err)
	}
	if cv.values[119] != 200 {
		t.Fatalf("CV119 = %d after test, want 200", cv.values[119])
	}
	if cv.values[120] != 100 {
		t.Fatalf("CV120 = %d after test, want 100", cv.values[120])
	}
}
