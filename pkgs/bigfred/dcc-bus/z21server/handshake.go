package z21server

// Roco Z21 identity advertised to stock handset apps (black Z21, FW 1.24).
const (
	HwTypeZ21Black     uint32 = 0x00000201 // D_HWT_Z21_NEW — retail “black Z21” (2013)
	FirmwareBCD        uint32 = 0x00000124   // BCD 1.24 in LAN_GET_HWINFO
	FirmwareVersionMSB byte   = 0x01
	FirmwareVersionLSB byte   = 0x24
	XBusProtocolVersion byte  = 0x36         // X-Bus V3.6
	CmdStationIDZ21    byte   = 0x12         // Z21 device family

	capDCC           byte = 0x01
	capLocoCmds      byte = 0x10
	capAccessoryCmds byte = 0x20
)

func (s *Server) effectiveSerial() uint32 {
	if s.cfg.Serial != 0 {
		return s.cfg.Serial
	}
	return rocoVirtualSerial(s.cfg.LayoutID, s.cfg.CommandStationID)
}

func (s *Server) effectiveHwType() uint32 {
	if s.cfg.HwType != 0 {
		return s.cfg.HwType
	}
	return HwTypeZ21Black
}

func (s *Server) effectiveFirmwareBCD() uint32 {
	if s.cfg.FirmwareBCD != 0 {
		return s.cfg.FirmwareBCD
	}
	return FirmwareBCD
}

// handshakeReply returns a LAN/X-BUS reply for identity and status probes.
func (s *Server) handshakeReply(pkt []byte) ([]byte, bool) {
	_, header, ok := packetHeader(pkt)
	if !ok {
		return nil, false
	}
	switch header {
	case HeaderGetSerialNumber:
		return buildSerialReply(s.effectiveSerial()), true
	case HeaderGetHWInfo:
		return buildHWInfoReply(s.effectiveHwType(), s.effectiveFirmwareBCD()), true
	case HeaderSystemStateGetData:
		return buildSystemStateReply(s.effectiveSystemState()), true
	case HeaderGetBroadcastFlags:
		return buildReply(HeaderGetBroadcastFlags, make([]byte, 4)), true
	case HeaderGetCode:
		return buildReply(HeaderGetCode, []byte{0x00}), true // Z21_NO_LOCK — full retail Z21
	case HeaderXBus:
		return s.xbusHandshakeReply(pkt)
	default:
		return nil, false
	}
}

func (s *Server) xbusHandshakeReply(pkt []byte) ([]byte, bool) {
	if len(pkt) < 7 {
		return nil, false
	}
	switch pkt[4] {
	case 0x21:
		switch pkt[5] {
		case 0x21:
			return buildGetVersionReply(), true
		case 0x24:
			return buildStatusChangedReply(), true
		}
	case 0xF1:
		if pkt[5] == 0x0A {
			return buildFirmwareVersionReply(), true
		}
	}
	return nil, false
}

func buildGetVersionReply() []byte {
	x := []byte{0x63, 0x21, XBusProtocolVersion, CmdStationIDZ21}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func buildFirmwareVersionReply() []byte {
	x := []byte{0xF3, 0x0A, FirmwareVersionMSB, FirmwareVersionLSB}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func buildStatusChangedReply() []byte {
	// Track on, no emergency stop, no short, no programming mode.
	x := []byte{0x62, 0x22, 0x00}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

// rocoVirtualSerial returns a stable serial in the range seen on retail Z21 units.
func rocoVirtualSerial(layoutID, commandStationID uint) uint32 {
	return uint32(258_000_000 + layoutID*1000 + commandStationID)
}
