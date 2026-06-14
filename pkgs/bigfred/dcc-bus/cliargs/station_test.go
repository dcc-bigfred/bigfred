package cliargs

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

func TestCommandStationFromFlags(t *testing.T) {
	cs, err := CommandStationFromFlags(3, "Z21", "z21", "udp://192.168.0.111:21105", 128)
	if err != nil {
		t.Fatal(err)
	}
	if cs.ID != 3 || cs.Kind != domain.CommandStationKindZ21 || cs.SpeedSteps != 128 {
		t.Fatalf("got %+v", cs)
	}
}

func TestAppendStationFlags(t *testing.T) {
	args := AppendStationFlags(nil, domain.CommandStation{
		Name: "Main", Kind: domain.CommandStationKindZ21,
		ConnectionURI: "udp://host:21105", SpeedSteps: 28,
	})
	joined := stringsJoin(args)
	for _, want := range []string{"--station-name", "Main", "--station-kind", "z21", "--station-uri", "udp://host:21105", "--speed-steps", "28"} {
		if !contains(args, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
}

func stringsJoin(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}
