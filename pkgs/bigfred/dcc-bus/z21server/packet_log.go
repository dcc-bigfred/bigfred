package z21server

import (
	"encoding/hex"
	"fmt"

	"github.com/sirupsen/logrus"
)

// PacketName returns a stable, human-readable Z21 packet label for logs.
func PacketName(pkt []byte) string {
	if len(pkt) < 4 {
		return "truncated"
	}
	dataLen, header, ok := packetHeader(pkt)
	if !ok {
		return "invalid_header"
	}
	if int(dataLen) != len(pkt) {
		return fmt.Sprintf("%s+batch(%d/%d)", lanHeaderName(header), len(pkt), dataLen)
	}
	if header == HeaderXBus {
		return xbusPacketName(pkt)
	}
	return lanHeaderName(header)
}

func lanHeaderName(header uint16) string {
	switch header {
	case HeaderGetSerialNumber:
		return "LAN_GET_SERIAL_NUMBER"
	case HeaderLogoff:
		return "LAN_LOGOFF"
	case HeaderSetBroadcastFlags:
		return "LAN_SET_BROADCASTFLAGS"
	case HeaderGetBroadcastFlags:
		return "LAN_GET_BROADCASTFLAGS"
	case HeaderSystemStateData:
		return "LAN_SYSTEMSTATE_DATACHANGED"
	case HeaderSystemStateGetData:
		return "LAN_SYSTEMSTATE_GETDATA"
	case HeaderGetHWInfo:
		return "LAN_GET_HWINFO"
	case HeaderGetCode:
		return "LAN_GET_CODE"
	case HeaderRMBusGetData:
		return "LAN_RMBUS_GETDATA"
	case HeaderRMBusDataChanged:
		return "LAN_RMBUS_DATACHANGED"
	case HeaderGetLocoMode:
		return "LAN_GET_LOCOMODE"
	case HeaderLocoNetFromLAN:
		return "LAN_LOCONET_FROM_LAN"
	case HeaderLanKeepalive:
		return "LAN_APP_KEEPALIVE"
	case HeaderLanSessionProbe:
		return "LAN_APP_SESSION_PROBE"
	default:
		return fmt.Sprintf("LAN_0x%04X", header)
	}
}

func xbusPacketName(pkt []byte) string {
	if len(pkt) < 6 {
		return "LAN_X_truncated"
	}
	switch pkt[4] {
	case 0x21:
		switch pkt[5] {
		case 0x21:
			return "LAN_X_GET_VERSION"
		case 0x24:
			return "LAN_X_GET_STATUS"
		case 0x80:
			return "LAN_X_SET_TRACK_POWER_OFF"
		case 0x81:
			return "LAN_X_SET_TRACK_POWER_ON"
		default:
			return fmt.Sprintf("LAN_X_0x21_0x%02X", pkt[5])
		}
	case 0x61:
		switch pkt[5] {
		case 0x00:
			return "LAN_X_BC_TRACK_POWER_OFF"
		case 0x01:
			return "LAN_X_BC_TRACK_POWER_ON"
		case 0x02:
			return "LAN_X_BC_PROGRAMMING_MODE"
		case 0x08:
			return "LAN_X_BC_TRACK_SHORT_CIRCUIT"
		case 0x12:
			return "LAN_X_CV_NACK_SC"
		case 0x13:
			return "LAN_X_CV_NACK"
		case 0x82:
			return "LAN_X_UNKNOWN_COMMAND"
		default:
			return fmt.Sprintf("LAN_X_BC_0x%02X", pkt[5])
		}
	case 0x62:
		return "LAN_X_STATUS_CHANGED"
	case 0x80:
		return "LAN_X_SET_STOP"
	case 0x81:
		return "LAN_X_BC_STOPPED"
	case 0xE3:
		if len(pkt) > 5 && pkt[5] == 0xF0 {
			return "LAN_X_GET_LOCO_INFO"
		}
		return fmt.Sprintf("LAN_X_0xE3_0x%02X", pkt[5])
	case 0xE4:
		if len(pkt) > 5 {
			switch pkt[5] {
			case 0xF8:
				return "LAN_X_SET_LOCO_FUNCTION"
			default:
				if pkt[5] >= 0x20 && pkt[5] <= 0x29 {
					return "LAN_X_SET_LOCO_FUNCTION_GROUP"
				}
				if pkt[5]&0xF0 == 0x10 {
					return "LAN_X_SET_LOCO_DRIVE"
				}
			}
			return fmt.Sprintf("LAN_X_0xE4_0x%02X", pkt[5])
		}
		return "LAN_X_SET_LOCO"
	case 0xE6:
		if len(pkt) > 5 && pkt[5] == 0x30 {
			if _, cvWire, _, ok := parsePOMWriteByte(pkt); ok {
				if isPairingCVWire(cvWire) {
					return fmt.Sprintf("LAN_X_CV_POM_WRITE_BYTE(cvWire=%d)", cvWire)
				}
				return fmt.Sprintf("LAN_X_CV_POM_WRITE_BYTE(cvWire=%d)", cvWire)
			}
			if len(pkt) > 8 {
				opt := pkt[8] & 0xFC
				switch opt {
				case 0xE4:
					return "LAN_X_CV_POM_READ_BYTE"
				case 0xEC:
					return "LAN_X_CV_POM_WRITE_BYTE"
				default:
					return fmt.Sprintf("LAN_X_CV_POM_0x%02X", opt)
				}
			}
			return "LAN_X_CV_POM"
		}
		return fmt.Sprintf("LAN_X_0xE6_0x%02X", pkt[5])
	case 0xEF:
		return "LAN_X_LOCO_INFO"
	case 0x23:
		return "LAN_X_CV_READ"
	case 0x24:
		return "LAN_X_CV_WRITE"
	case 0x64:
		return "LAN_X_CV_RESULT"
	case 0xF1:
		return "LAN_X_GET_FIRMWARE_VERSION"
	case 0xF3:
		return "LAN_X_FIRMWARE_VERSION"
	default:
		return fmt.Sprintf("LAN_X_0x%02X", pkt[4])
	}
}

// isDiscoveryHandshakePacket reports LAN/X-BUS probes answered without pairing.
func isDiscoveryHandshakePacket(pkt []byte) bool {
	_, header, ok := packetHeader(pkt)
	if !ok {
		return false
	}
	switch header {
	case HeaderGetSerialNumber,
		HeaderGetHWInfo,
		HeaderSystemStateGetData,
		HeaderGetBroadcastFlags,
		HeaderGetCode,
		HeaderLanKeepalive,
		HeaderLanSessionProbe:
		return true
	case HeaderXBus:
		if len(pkt) < 6 {
			return false
		}
		switch pkt[4] {
		case 0x21:
			return pkt[5] == 0x21 || pkt[5] == 0x24
		case 0xF1:
			return pkt[5] == 0x0A
		}
	}
	return false
}

func (s *Server) logRx(clientKey string, pkt []byte, paired bool, action string) {
	if s.log == nil {
		return
	}
	level := logrus.DebugLevel
	if isDiscoveryHandshakePacket(pkt) {
		level = logrus.DebugLevel
	}
	if !s.log.IsLevelEnabled(level) {
		return
	}
	payload := hex.EncodeToString(pkt)
	if len(payload) > 64 {
		payload = payload[:64] + "…"
	}
	s.log.WithFields(logrus.Fields{
		"dir":     "RX",
		"client":  clientKey,
		"packet":  PacketName(pkt),
		"paired":  paired,
		"action":  action,
		"len":     len(pkt),
		"payload": payload,
	}).Log(level, "z21 udp")
}

func (s *Server) logTx(clientKey string, pkt []byte) {
	if s.log == nil {
		return
	}
	level := logrus.DebugLevel
	if isDiscoveryHandshakePacket(pkt) {
		level = logrus.DebugLevel
	}
	if !s.log.IsLevelEnabled(level) {
		return
	}
	payload := hex.EncodeToString(pkt)
	if len(payload) > 64 {
		payload = payload[:64] + "…"
	}
	s.log.WithFields(logrus.Fields{
		"dir":     "TX",
		"client":  clientKey,
		"packet":  PacketName(pkt),
		"len":     len(pkt),
		"payload": payload,
	}).Log(level, "z21 udp")
}

func (s *Server) logUnhandled(clientKey string, pkt []byte, paired bool, reason string) {
	if s.log == nil {
		return
	}
	s.log.WithFields(logrus.Fields{
		"client":  clientKey,
		"packet":  PacketName(pkt),
		"paired":  paired,
		"reason":  reason,
		"len":     len(pkt),
		"payload": hex.EncodeToString(pkt),
	}).Info("z21 unhandled packet")
}
