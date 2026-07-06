package contract

import "testing"

func TestUISpeedFromWire(t *testing.T) {
	tests := []struct {
		wire, want uint8
	}{
		{0, 0},
		{1, 0},
		{2, 2},
		{127, 127},
	}
	for _, tc := range tests {
		got := UISpeedFromWire(tc.wire)
		if got != tc.want {
			t.Fatalf("UISpeedFromWire(%d) = %d, want %d", tc.wire, got, tc.want)
		}
	}
}
