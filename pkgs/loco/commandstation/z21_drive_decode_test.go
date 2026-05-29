package commandstation

import "testing"

func TestDecodeLocoDriveFromLocoInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		db2     byte
		db3     byte
		speed   uint8
		forward bool
	}{
		{
			name:    "128-step stop forward",
			db2:     0x04,
			db3:     0x80,
			speed:   0,
			forward: true,
		},
		{
			name:    "128-step stop reverse",
			db2:     0x04,
			db3:     0x00,
			speed:   0,
			forward: false,
		},
		{
			name:    "128-step mid forward",
			db2:     0x04,
			db3:     0xC0,
			speed:   64,
			forward: true,
		},
		{
			name:    "128-step max forward",
			db2:     0x04,
			db3:     0xFF,
			speed:   127,
			forward: true,
		},
		{
			name:    "128-step e-stop",
			db2:     0x04,
			db3:     0x01,
			speed:   1,
			forward: false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			speed, forward := decodeLocoDriveFromLocoInfo(tc.db2, tc.db3)
			if speed != tc.speed || forward != tc.forward {
				t.Fatalf("decodeLocoDriveFromLocoInfo(%#02x, %#02x) = (%d, %v), want (%d, %v)",
					tc.db2, tc.db3, speed, forward, tc.speed, tc.forward)
			}
		})
	}
}
