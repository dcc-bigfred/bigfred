package z21server

import "testing"

func TestPacketNameLANHeaders(t *testing.T) {
	cases := []struct {
		pkt  []byte
		want string
	}{
		{[]byte{0x04, 0x00, 0x10, 0x00}, "LAN_GET_SERIAL_NUMBER"},
		{[]byte{0x04, 0x00, 0x1A, 0x00}, "LAN_GET_HWINFO"},
		{[]byte{0x04, 0x00, 0x85, 0x00}, "LAN_SYSTEMSTATE_GETDATA"},
		{[]byte{0x04, 0x00, 0x30, 0x00}, "LAN_LOGOFF"},
	}
	for _, tc := range cases {
		if got := PacketName(tc.pkt); got != tc.want {
			t.Fatalf("PacketName(% X) = %q, want %q", tc.pkt, got, tc.want)
		}
	}
}

func TestPacketNamePOMWrite(t *testing.T) {
	pkt := buildPOMWriteByte(3, cvWireCV3, 155)
	if got := PacketName(pkt); got != "LAN_X_CV_POM_WRITE_BYTE(cvWire=2)" {
		t.Fatalf("PacketName(POM CV3) = %q", got)
	}
}

func TestPacketNameGetVersion(t *testing.T) {
	pkt := []byte{0x07, 0x00, 0x40, 0x00, 0x21, 0x21, 0x00}
	if got := PacketName(pkt); got != "LAN_X_GET_VERSION" {
		t.Fatalf("PacketName(GET_VERSION) = %q", got)
	}
}
