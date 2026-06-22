package decoders

import (
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name    string
		cv8     int
		cv110   int
		set110  bool
		wantErr bool
	}{
		{"railbox locomotive", ManufacturerRailBOX, railboxProductRB2300, true, false},
		{"railbox wagon", ManufacturerRailBOX, 0, true, true},
		{"esu", ManufacturerESU, 0, false, false},
		{"zimo", ManufacturerZIMO, 0, false, false},
		{"railbox wagon cv8=13", railboxWagonManufacturerCV8, 0, false, true},
		{"unknown", 99, 0, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[uint16]int{7: 10, 8: tt.cv8}
			if tt.set110 {
				values[110] = tt.cv110
			}
			cv := &fakeCV{values: values}
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

func TestIdentify(t *testing.T) {
	tests := []struct {
		name     string
		cv7      int
		cv8      int
		cv110    int
		set110   bool
		wantName string
		wantKind DecoderKind
	}{
		{"railbox locomotive", 110, ManufacturerRailBOX, railboxProductRB2300, true, "RailBOX RB23xx", DecoderRailBOX},
		{"railbox wagon", 5, ManufacturerRailBOX, 12, true, "RailBOX RB 2112", DecoderRailBOX},
		{"railbox without cv110", 5, ManufacturerRailBOX, 0, false, "RailBOX RB 2112", DecoderRailBOX},
		{"esu", 5, ManufacturerESU, 0, false, "ESU LokSound 5", DecoderESU},
		{"zimo", 14, ManufacturerZIMO, 0, false, "ZIMO MS/MN", DecoderZIMO},
		{"railbox wagon cv8=13", 31, railboxWagonManufacturerCV8, 0, false, "RailBOX RB 2112", DecoderRailBOX},
		{"unknown", 1, 99, 0, false, "unknown", DecoderUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[uint16]int{7: tt.cv7, 8: tt.cv8}
			if tt.set110 {
				values[110] = tt.cv110
			}
			cv := &fakeCV{values: values}
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

func TestDetectBrightness(t *testing.T) {
	tests := []struct {
		name       string
		cv8        int
		cv110      int
		set110     bool
		wantErr    bool
		wantRB2112 bool
	}{
		{"railbox locomotive", ManufacturerRailBOX, railboxProductRB2300, true, false, false},
		{"railbox wagon", ManufacturerRailBOX, 12, true, false, true},
		{"esu", ManufacturerESU, 0, false, false, false},
		{"zimo", ManufacturerZIMO, 0, false, true, false},
		{"railbox wagon cv8=13", railboxWagonManufacturerCV8, 0, false, false, true},
		{"unknown", 99, 0, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := map[uint16]int{7: 10, 8: tt.cv8}
			if tt.set110 {
				values[110] = tt.cv110
			}
			cv := &fakeCV{values: values}
			got, err := GetBrightnessImplementation(cv)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectBrightness: %v", err)
			}
			if got == nil {
				t.Fatal("expected decoder, got nil")
			}
			if tt.wantRB2112 {
				if _, ok := got.(*RailboxRB2112); !ok {
					t.Fatalf("expected *RailboxRB2112, got %T", got)
				}
			}
		})
	}
}
