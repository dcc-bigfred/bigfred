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

func TestIsTurnoutPoll(t *testing.T) {
	// WLANmaus turnout/accessory poll (LAN_X_GET_TURNOUT_INFO, X-Header 0x43).
	poll := []byte{0x09, 0x00, 0x40, 0x00, 0x43, 0x00, 0x00, 0x00, 0x81}
	if !isTurnoutPoll(poll) {
		t.Fatal("expected 0x43 frame to be classified as turnout poll")
	}
	getInfo := []byte{0x09, 0x00, 0x40, 0x00, 0xE3, 0xF0, 0x00, 0x1F, 0x0C}
	if isTurnoutPoll(getInfo) {
		t.Fatal("LAN_X_GET_LOCO_INFO must not be a turnout poll")
	}
	if isTurnoutPoll([]byte{0x04, 0x00, 0x10, 0x00}) {
		t.Fatal("non X-Bus header must not be a turnout poll")
	}
}

func TestIsDiscoveryHandshakePacket(t *testing.T) {
	serialReq := []byte{0x04, 0x00, 0x10, 0x00}
	serialReply := SerialReply(0x0f60cc55)
	if !isDiscoveryHandshakePacket(serialReq) || !isDiscoveryHandshakePacket(serialReply) {
		t.Fatal("LAN_GET_SERIAL_NUMBER should be discovery handshake")
	}
	drive := buildReply(HeaderXBus, []byte{0xE4, 0x12, 0x01, 0x02, 0x03})
	if isDiscoveryHandshakePacket(drive) {
		t.Fatal("drive packet should not be discovery handshake")
	}
}
