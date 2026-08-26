package withrottle

import (
	"strconv"
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
	}, 0, true, 128)
	if len(lines) < 3 {
		t.Fatalf("lines: %v", lines)
	}
	if lines[1] != "M0LS3<;>]\\[]\\[Bell" {
		t.Fatalf("labels: %q", lines[1])
	}
}

func TestBuildAcquireReplyUsesSnapshotSpeedAndDirection(t *testing.T) {
	lines := buildAcquireReply('0', 3, nil, 42, false, 128)
	wantV := "M0AS3<;>V" + strconv.Itoa(wireSpeedFromDCC(42, 128))
	wantR := "M0AS3<;>R0"
	var gotV, gotR string
	for _, line := range lines {
		if len(line) >= 10 && line[:9] == "M0AS3<;>V" {
			gotV = line
		}
		if len(line) >= 10 && line[:9] == "M0AS3<;>R" {
			gotR = line
		}
	}
	if gotV != wantV {
		t.Fatalf("speed line %q want %q", gotV, wantV)
	}
	if gotR != wantR {
		t.Fatalf("dir line %q want %q", gotR, wantR)
	}
}

func TestBuildRosterLineUsesDisplayName(t *testing.T) {
	session := &contract.RemoteSessionWire{AllowAllVehicles: true, UserID: 9}
	allowed := contract.AllowedVehicles{
		Vehicles: []contract.AllowedVehicle{
			{VehicleID: "V-1", DisplayName: "ET22", Addr: 3, ControllerUserIDs: []uint{9}},
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
