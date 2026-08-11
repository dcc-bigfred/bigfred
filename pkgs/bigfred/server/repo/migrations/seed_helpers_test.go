package migrations

import "testing"

func TestIsMomentaryIcon(t *testing.T) {
	t.Parallel()
	cases := []struct {
		icon string
		want bool
	}{
		{"horn_high", true},
		{"horn_low", true},
		{"whistle", false},
		{"light", false},
		{"bell", false},
	}
	for _, tc := range cases {
		if got := isMomentaryIcon(tc.icon); got != tc.want {
			t.Errorf("isMomentaryIcon(%q) = %v, want %v", tc.icon, got, tc.want)
		}
	}
}

func TestMomentarySeedValues(t *testing.T) {
	t.Parallel()
	m, d := momentarySeedValues("horn_low")
	if m != 1 || d != defaultMomentaryDurationMs {
		t.Fatalf("horn: got momentary=%d durationMs=%d", m, d)
	}
	m, d = momentarySeedValues("light")
	if m != 0 || d != defaultMomentaryDurationMs {
		t.Fatalf("light: got momentary=%d durationMs=%d", m, d)
	}
}

func TestForceMomentarySeed(t *testing.T) {
	t.Parallel()
	force := forceMomentaryOverride{3: 1000, 4: 1000}
	fn := templateFunctionSeed{3, "F3", "unspecified"}
	m, d := fn.momentaryValues(force)
	if m != 1 || d != 1000 {
		t.Fatalf("force momentary: got momentary=%d durationMs=%d", m, d)
	}
	fn = templateFunctionSeed{0, "F0", "light"}
	m, d = fn.momentaryValues(force)
	if m != 0 || d != defaultMomentaryDurationMs {
		t.Fatalf("non-force: got momentary=%d durationMs=%d", m, d)
	}
}
