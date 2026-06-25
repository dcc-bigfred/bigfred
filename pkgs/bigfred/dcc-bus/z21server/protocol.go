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
)

// Virtual hardware identity advertised to stock Z21 apps.
const (
	HwTypeZ21Small    uint32 = 0x00000203 // D_HWT_z21_SMALL
	FirmwareVersion12 uint32 = 0x00000120 // BCD 1.20
)

// IdleEvictAfter is Z21 §1.1 — no UDP for this long removes the client.
const IdleEvictAfter = 60

// SweeperInterval is how often the server scans for idle clients.
const SweeperInterval = 15

func packetHeader(pkt []byte) (dataLen, header uint16, ok bool) {
	if len(pkt) < 4 {
		return 0, 0, false
	}
	return binary.LittleEndian.Uint16(pkt[0:2]),
		binary.LittleEndian.Uint16(pkt[2:4]),
		true
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

// buildSystemStateReply returns track on, no programming mode, no short.
func buildSystemStateReply() []byte {
	data := make([]byte, 16)
	// VCCVoltage 12000 mV — non-zero so apps treat track as live.
	binary.LittleEndian.PutUint16(data[8:10], 12000)
	binary.LittleEndian.PutUint16(data[10:12], 12000)
	// CentralState / CentralStateEx / reserved / Capabilities left zero.
	return buildReply(HeaderSystemStateData, data)
}

func isHandshakeHeader(header uint16) bool {
	switch header {
	case HeaderGetSerialNumber, HeaderGetHWInfo, HeaderSystemStateGetData, HeaderGetBroadcastFlags:
		return true
	default:
		return false
	}
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
