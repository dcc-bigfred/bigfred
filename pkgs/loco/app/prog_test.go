package app

import (
	"testing"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func TestProgModeForLoco(t *testing.T) {
	if got := progModeForLoco(0); got != commandstation.ProgrammingTrackMode {
		t.Fatalf("progModeForLoco(0) = %q, want %q", got, commandstation.ProgrammingTrackMode)
	}
	if got := progModeForLoco(3); got != commandstation.MainTrackMode {
		t.Fatalf("progModeForLoco(3) = %q, want %q", got, commandstation.MainTrackMode)
	}
}
