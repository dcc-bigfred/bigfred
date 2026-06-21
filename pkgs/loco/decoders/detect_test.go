package decoders

import (
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name    string
		cv8     int
		wantErr bool
	}{
		{"railbox", ManufacturerRailBOX, false},
		{"esu", ManufacturerESU, false},
		{"zimo", ManufacturerZIMO, false},
		{"unknown", 99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := &fakeCV{values: map[uint16]int{7: 10, 8: tt.cv8}}
			got, err := Detect(cv)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if got == nil {
				t.Fatal("expected decoder, got nil")
			}
		})
	}
}

func TestDetectReadsCV8(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{7: 5, 8: ManufacturerESU}}
	decoder, err := Detect(cv)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if _, ok := decoder.(*LokSoundv5); !ok {
		t.Fatalf("expected *LokSoundv5, got %T", decoder)
	}
}

func TestIdentify(t *testing.T) {
	tests := []struct {
		name     string
		cv7      int
		cv8      int
		wantName string
		wantKind DecoderKind
	}{
		{"railbox", 110, ManufacturerRailBOX, "RailBOX RB23xx", DecoderRailBOX},
		{"esu", 5, ManufacturerESU, "ESU LokSound 5", DecoderESU},
		{"zimo", 14, ManufacturerZIMO, "ZIMO MS/MN", DecoderZIMO},
		{"unknown", 1, 99, "unknown", DecoderUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := &fakeCV{values: map[uint16]int{7: tt.cv7, 8: tt.cv8}}
			id, err := Identify(cv)
			if err != nil {
				t.Fatalf("Identify: %v", err)
			}
			if id.Name != tt.wantName {
				t.Fatalf("Name = %q, want %q", id.Name, tt.wantName)
			}
			if id.Kind != tt.wantKind {
				t.Fatalf("Kind = %v, want %v", id.Kind, tt.wantKind)
			}
			if id.SoftwareVersion != tt.cv7 {
				t.Fatalf("SoftwareVersion = %d, want %d", id.SoftwareVersion, tt.cv7)
			}
			if id.ManufacturerID != tt.cv8 {
				t.Fatalf("ManufacturerID = %d, want %d", id.ManufacturerID, tt.cv8)
			}
		})
	}
}
