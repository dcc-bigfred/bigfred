package contract

import "testing"

func TestClampSpeedForControllerLimit(t *testing.T) {
	tests := []struct {
		speed, max, limit, want uint8
	}{
		{100, 127, 0, 100},
		{100, 127, 100, 100},
		{100, 127, 80, 100},
		{120, 127, 80, 102},
		{120, 127, 50, 64},
		{10, 28, 80, 10},
		{10, 127, 1, 2},
		{10, 28, 1, 2},
	}
	for _, tc := range tests {
		got := ClampSpeedForControllerLimit(tc.speed, tc.max, tc.limit)
		if got != tc.want {
			t.Fatalf("Clamp(%d,%d,%d) = %d, want %d", tc.speed, tc.max, tc.limit, got, tc.want)
		}
	}
}
