package decoders

import (
	"testing"
)

type fakeCV struct {
	values map[uint16]int
}

func (f *fakeCV) ReadCV(num uint16) (int, error) {
	return f.values[num], nil
}

func (f *fakeCV) WriteCV(num uint16, value int) error {
	f.values[num] = value
	return nil
}

func TestPercentToCVAndBack(t *testing.T) {
	tests := []struct {
		name   string
		maxCV  uint8
		cvNum  uint16
		percent uint8
		wantCV int
	}{
		{"railbox 0%", railboxRB23xxVolumeMaxCV, railboxRB23xxVolumeCV, 0, 0},
		{"railbox 50%", railboxRB23xxVolumeMaxCV, railboxRB23xxVolumeCV, 50, 32},
		{"railbox 100%", railboxRB23xxVolumeMaxCV, railboxRB23xxVolumeCV, 100, 64},
		{"esu 0%", lokSoundV5VolumeMaxCV, lokSoundV5VolumeCV, 0, 0},
		{"esu 50%", lokSoundV5VolumeMaxCV, lokSoundV5VolumeCV, 50, 96},
		{"esu 100%", lokSoundV5VolumeMaxCV, lokSoundV5VolumeCV, 100, 192},
		{"zimo 0%", zimoMS450VolumeMaxCV, zimoMS450VolumeCV, 0, 0},
		{"zimo 50%", zimoMS450VolumeMaxCV, zimoMS450VolumeCV, 50, 33},
		{"zimo 100%", zimoMS450VolumeMaxCV, zimoMS450VolumeCV, 100, 65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentToCV(tt.percent, tt.maxCV)
			if got != tt.wantCV {
				t.Fatalf("percentToCV(%d, %d) = %d, want %d", tt.percent, tt.maxCV, got, tt.wantCV)
			}
			if tt.percent == 0 || tt.percent == 100 {
				back := cvToPercent(got, int(tt.maxCV))
				if back != tt.percent {
					t.Fatalf("cvToPercent(%d, %d) = %d, want %d", got, tt.maxCV, back, tt.percent)
				}
			}
		})
	}
}

func TestLokSoundv5SetGetVolume(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewLokSoundv5(cv)

	if err := d.SetVolume(75); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if cv.values[lokSoundV5VolumeCV] != 144 {
		t.Fatalf("written CV = %d, want 144", cv.values[lokSoundV5VolumeCV])
	}

	got, err := d.GetVolume()
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if got != 75 {
		t.Fatalf("GetVolume = %d, want 75", got)
	}
}

func TestZIMOMS450SetGetVolume(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewZIMOMS450(cv)

	if err := d.SetVolume(40); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if cv.values[zimoMS450VolumeCV] != 26 {
		t.Fatalf("written CV = %d, want 26", cv.values[zimoMS450VolumeCV])
	}

	got, err := d.GetVolume()
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if got != 40 {
		t.Fatalf("GetVolume = %d, want 40", got)
	}
}

func TestRailboxRB23xxSetGetVolume(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewRailboxRB23xx(WithCVAccess(cv))

	if err := d.SetVolume(25); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if cv.values[railboxRB23xxVolumeCV] != 16 {
		t.Fatalf("written CV = %d, want 16", cv.values[railboxRB23xxVolumeCV])
	}

	got, err := d.GetVolume()
	if err != nil {
		t.Fatalf("GetVolume: %v", err)
	}
	if got != 25 {
		t.Fatalf("GetVolume = %d, want 25", got)
	}
}

func TestSetVolumeRejectsOver100(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	d := NewLokSoundv5(cv)
	if err := d.SetVolume(101); err == nil {
		t.Fatal("expected error for percent > 100")
	}
}
