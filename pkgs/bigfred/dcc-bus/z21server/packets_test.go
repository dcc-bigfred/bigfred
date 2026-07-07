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

// TestDriveSpeedEncodeDecodeRoundtrip confirms parseSetLocoDrive returns
// payload/UI speed (Fix S2): the same encoding used when replying with
// buildLocoInfoReply must round-trip through decodeDriveDB3.
func TestDriveSpeedEncodeDecodeRoundtrip(t *testing.T) {
	t.Parallel()
	proto := byte(3) // 128-step LocoNet-style encoding
	for _, payload := range []uint8{0, 1, 2, 10, 127} {
		payload := payload
		t.Run("", func(t *testing.T) {
			t.Parallel()
			db3 := encodeDriveDB3(payload, true, proto)
			got, forward := decodeDriveDB3(proto, db3)
			if !forward {
				t.Fatal("forward bit lost")
			}
			if got != payload {
				t.Fatalf("payload %d: decode = %d", payload, got)
			}
		})
	}
}

// TestDriveSpeed28StepBoundary confirms the Z21 28-step encoding maps stop to
// payload 0 and preserves moving steps without exceeding the 28-step range.
// The 28-step DB3 encoding is lossy (raw<=1 → stop, raw<=3 → step 1, else raw-3).
func TestDriveSpeed28StepBoundary(t *testing.T) {
	t.Parallel()
	proto := byte(2) // 28-step
	db3 := encodeDriveDB3(0, true, proto)
	got, _ := decodeDriveDB3(proto, db3)
	if got != 0 {
		t.Fatalf("28-step payload 0: decode = %d, want 0", got)
	}
	// Payload 1 is the first moving notch in 28-step mode.
	db3 = encodeDriveDB3(1, true, proto)
	got, _ = decodeDriveDB3(proto, db3)
	if got != 1 {
		t.Fatalf("28-step payload 1: decode = %d, want 1 (first notch)", got)
	}
	for _, payload := range []uint8{2, 3, 10, 28} {
		payload := payload
		t.Run("", func(t *testing.T) {
			t.Parallel()
			db3 := encodeDriveDB3(payload, true, proto)
			got, _ := decodeDriveDB3(proto, db3)
			if got == 0 {
				t.Fatalf("28-step payload %d: decode = 0, want moving step", payload)
			}
			if got > 28 {
				t.Fatalf("28-step payload %d: decode = %d, exceeds 28", payload, got)
			}
		})
	}
}

func TestParseSetLocoFunctionZ21AppPacket(t *testing.T) {
	t.Parallel()
	pkt := mustHex(t, "0a004000e4f8001f5152")
	addr, fn, sw, ok := parseSetLocoFunction(pkt)
	if !ok || addr != 31 || fn != 17 || sw != funcSwitchOn {
		t.Fatalf("parseSetLocoFunction = (%d, %d, %v, %v)", addr, fn, sw, ok)
	}
}

func TestParseSetLocoFunctionToggle(t *testing.T) {
	t.Parallel()
	// WLANmaus F1 toggle: DB3 = 0x81 = 10_000001.
	pkt := mustHex(t, "0a004000e4f8001f8100")
	addr, fn, sw, ok := parseSetLocoFunction(pkt)
	if !ok || addr != 31 || fn != 1 || sw != funcSwitchToggle {
		t.Fatalf("parseSetLocoFunction toggle = (%d, %d, %v, %v)", addr, fn, sw, ok)
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
