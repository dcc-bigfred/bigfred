package decoders

import "testing"

func TestFactoryResetCVValue(t *testing.T) {
	tests := []struct {
		name    string
		kind    DecoderKind
		want    int
		wantErr bool
	}{
		{"railbox", DecoderRailBOX, 1, false},
		{"esu", DecoderESU, 8, false},
		{"zimo", DecoderZIMO, 8, false},
		{"unknown", DecoderUnknown, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FactoryResetCVValue(tt.kind)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("FactoryResetCVValue: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
