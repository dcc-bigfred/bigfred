package withrottle

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func TestBuildFunctionLabelLine(t *testing.T) {
	defs := []contract.FunctionDefinition{
		{Num: 0, Name: "Headlight"},
		{Num: 2, Name: "Whistle"},
	}
	got := buildFunctionLabelLine('0', "S3", defs)
	want := "M0LS3<;>]\\[Headlight]\\[]\\[Whistle"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildAcquireReplyIncludesLabels(t *testing.T) {
	lines := buildAcquireReply('0', 3, []contract.FunctionDefinition{
		{Num: 1, Name: "Bell"},
	})
	if len(lines) < 3 {
		t.Fatalf("lines: %v", lines)
	}
	if lines[1] != "M0LS3<;>]\\[]\\[Bell" {
		t.Fatalf("labels: %q", lines[1])
	}
}

func TestBuildRosterLineUsesDisplayName(t *testing.T) {
	session := &contract.RemoteSessionWire{AllowAllVehicles: true}
	allowed := contract.AllowedVehicles{
		Vehicles: []contract.AllowedVehicle{
			{VehicleID: "V-1", DisplayName: "ET22", Addr: 3},
		},
	}
	line := BuildRosterLine(session, allowed, 10239, true)
	if !containsSubstr(line, "ET22") {
		t.Fatalf("line %q should contain display name", line)
	}
	if containsSubstr(line, "V-1") {
		t.Fatalf("line %q should not contain vehicle id", line)
	}
}

func containsSubstr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexSubstr(s, sub) >= 0)
}

func indexSubstr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
