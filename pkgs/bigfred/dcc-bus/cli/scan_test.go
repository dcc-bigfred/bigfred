package cli

import (
	"testing"

	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func TestBuildScanAutodetections(t *testing.T) {
	cases := []struct {
		name           string
		supportsSerial bool
		lanPrefix      string
		wantLen        int
		wantSerial     bool
		wantLAN        bool
	}{
		{"hub with LAN", true, "192.168.0", 3, true, true},
		{"hub without LAN", true, "", 1, true, false},
		{"phone with LAN", false, "192.168.0", 2, false, true},
		{"phone without LAN", false, "", 0, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildScanAutodetections(tc.supportsSerial, tc.lanPrefix)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			hasSerial := false
			hasLAN := false
			for _, s := range got {
				switch s.(type) {
				case commandstation.LocoNetSerialAutodetection:
					hasSerial = true
				case commandstation.LocoNetTCPAutodetection, commandstation.Z21Autodetection:
					hasLAN = true
				}
			}
			if hasSerial != tc.wantSerial {
				t.Fatalf("serial = %v, want %v", hasSerial, tc.wantSerial)
			}
			if hasLAN != tc.wantLAN {
				t.Fatalf("lan = %v, want %v", hasLAN, tc.wantLAN)
			}
		})
	}
}
