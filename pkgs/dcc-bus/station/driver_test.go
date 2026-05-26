package station

import "testing"

func TestParseHostPort(t *testing.T) {
	cases := []struct {
		name        string
		uri         string
		scheme      string
		defaultPort uint16
		wantHost    string
		wantPort    uint16
		wantErr     bool
	}{
		{"plain host:port", "192.168.1.10:21105", "udp", 21105, "192.168.1.10", 21105, false},
		{"with scheme", "udp://192.168.1.10:21105", "udp", 21105, "192.168.1.10", 21105, false},
		{"host only uses default", "192.168.1.10", "udp", 21105, "192.168.1.10", 21105, false},
		{"empty rejected", "", "udp", 21105, "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := parseHostPort(tc.uri, tc.scheme, tc.defaultPort)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if host != tc.wantHost || port != tc.wantPort {
				t.Fatalf("got (%q, %d) want (%q, %d)", host, port, tc.wantHost, tc.wantPort)
			}
		})
	}
}

func TestParseSerial(t *testing.T) {
	device, baud, err := parseSerial("serial:///dev/ttyUSB0:57600")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if device != "/dev/ttyUSB0" || baud != 57600 {
		t.Fatalf("got (%q, %d)", device, baud)
	}

	device, baud, _ = parseSerial("/dev/ttyUSB0")
	if device != "/dev/ttyUSB0" || baud != 57600 {
		t.Fatalf("device-only: got (%q, %d)", device, baud)
	}
}
