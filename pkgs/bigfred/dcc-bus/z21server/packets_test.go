package z21server

import (
	"encoding/hex"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseGetLocoInfoZ21AppPacket(t *testing.T) {
	t.Parallel()
	pkt := mustHex(t, "09004000e3f0001f0c")
	addr, ok := parseGetLocoInfo(pkt)
	if !ok || addr != 31 {
		t.Fatalf("parseGetLocoInfo = (%d, %v), want (31, true)", addr, ok)
	}
}

func TestParseSetLocoDriveZ21AppPacket(t *testing.T) {
	t.Parallel()
	pkt := mustHex(t, "0a004000e413001f00e8")
	addr, speed, forward, ok := parseSetLocoDrive(pkt)
	if !ok || addr != 31 || speed != 0 || forward {
		t.Fatalf("parseSetLocoDrive = (%d, %d, %v, %v)", addr, speed, forward, ok)
	}
}

func TestParseSetLocoFunctionZ21AppPacket(t *testing.T) {
	t.Parallel()
	pkt := mustHex(t, "0a004000e4f8001f5152")
	addr, fn, on, ok := parseSetLocoFunction(pkt)
	if !ok || addr != 31 || fn != 17 || !on {
		t.Fatalf("parseSetLocoFunction = (%d, %d, %v, %v)", addr, fn, on, ok)
	}
}

func TestBuildLocoInfoReplyRoundtripFields(t *testing.T) {
	t.Parallel()
	reply := buildLocoInfoReply(31, contract.LocoStateWire{
		Address:   31,
		Speed:     5,
		Forward:   true,
		Functions: []bool{false, false, true},
	}, 128)
	if len(reply) != 15 {
		t.Fatalf("len = %d, want 15 (9-byte X-BUS + 4 header + XOR)", len(reply))
	}
	if reply[4] != 0xEF {
		t.Fatalf("X-Header = %#02x, want 0xEF", reply[4])
	}
	// F2 active → DB4 bit 1 per Z21 LAN_X_LOCO_INFO layout.
	if reply[9] != 0x02 {
		t.Fatalf("DB4 = %#02x, want 0x02 for F2", reply[9])
	}
}

func TestEncodeFunctionBytesZ21Layout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		fn      int
		wantIdx int
		want    byte
	}{
		{0, 9, 0x10},
		{1, 9, 0x01},
		{2, 9, 0x02},
		{3, 9, 0x04},
		{4, 9, 0x08},
		{5, 10, 0x01},
		{13, 11, 0x01},
		{21, 12, 0x01},
		{29, 13, 0x01},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			fns := make([]bool, tc.fn+1)
			fns[tc.fn] = true
			reply := buildLocoInfoReply(1, contract.LocoStateWire{
				Address:   1,
				Functions: fns,
			}, 128)
			if got := reply[tc.wantIdx]; got != tc.want {
				t.Fatalf("F%d: byte[%d]=%#02x, want %#02x", tc.fn, tc.wantIdx, got, tc.want)
			}
		})
	}
}
