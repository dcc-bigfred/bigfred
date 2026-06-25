package z21server

import "encoding/binary"

// Z21 LAN command headers (little-endian uint16 at bytes 2–3).
const (
	HeaderGetSerialNumber    uint16 = 0x0010
	HeaderLogoff             uint16 = 0x0030
	HeaderSetBroadcastFlags  uint16 = 0x0050
	HeaderGetBroadcastFlags  uint16 = 0x0051
	HeaderXBus               uint16 = 0x0040
	HeaderSystemStateData    uint16 = 0x0084
	HeaderSystemStateGetData uint16 = 0x0085
	HeaderGetHWInfo          uint16 = 0x001A
	HeaderGetCode            uint16 = 0x0018
	HeaderRMBusGetData       uint16 = 0x0081
	HeaderRMBusDataChanged   uint16 = 0x0080
	HeaderGetLocoMode        uint16 = 0x0060
	HeaderLocoNetFromLAN     uint16 = 0x00A2
	// Undocumented Roco Z21 mobile app keepalive / session probes (not in LAN spec).
	HeaderLanKeepalive  uint16 = 0x0035
	HeaderLanSessionProbe uint16 = 0x0036
)

// IdleEvictAfter is Z21 §1.1 — no UDP for this long removes a non-sticky client.
const IdleEvictAfter = 60

// StickySessionIdleEvictAfter is the idle window before an IP-sticky paired
// handset is unpaired (seconds).
const StickySessionIdleEvictAfter = 30 * 60

// SweeperInterval is how often the server scans clients.
const SweeperInterval = 3

// ClientsPublishMinInterval limits Redis/WS snapshot publish rate (seconds).
const ClientsPublishMinInterval = 2

func packetHeader(pkt []byte) (dataLen, header uint16, ok bool) {
	if len(pkt) < 4 {
		return 0, 0, false
	}
	return binary.LittleEndian.Uint16(pkt[0:2]),
		binary.LittleEndian.Uint16(pkt[2:4]),
		true
}

// splitZ21Datagram splits a UDP payload into length-prefixed Z21 datasets (§1.3).
func splitZ21Datagram(b []byte) [][]byte {
	var out [][]byte
	for len(b) >= 4 {
		l := int(binary.LittleEndian.Uint16(b[0:2]))
		if l < 4 || l > len(b) {
			break
		}
		out = append(out, b[:l])
		b = b[l:]
	}
	if len(out) == 0 && len(b) > 0 {
		out = append(out, b)
	}
	return out
}

func buildReply(header uint16, data []byte) []byte {
	dataLen := uint16(4 + len(data))
	out := make([]byte, 4+len(data))
	binary.LittleEndian.PutUint16(out[0:2], dataLen)
	binary.LittleEndian.PutUint16(out[2:4], header)
	copy(out[4:], data)
	return out
}

func buildSerialReply(serial uint32) []byte {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, serial)
	return buildReply(HeaderGetSerialNumber, data)
}

func buildHWInfoReply(hwType, fwVersion uint32) []byte {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:4], hwType)
	binary.LittleEndian.PutUint32(data[4:8], fwVersion)
	return buildReply(HeaderGetHWInfo, data)
}

func isDriveHeader(header uint16, pkt []byte) bool {
	if header != HeaderXBus || len(pkt) < 6 {
		return false
	}
	switch pkt[4] {
	case 0xE4: // LAN_X_SET_LOCO_DRIVE, LAN_X_SET_LOCO_FUNCTION, …
		return pkt[5] == 0xF8 || pkt[5] == 0x13
	case 0xE3: // LAN_X_GET_LOCO_INFO
		return pkt[5] == 0xF0
	default:
		return false
	}
}
