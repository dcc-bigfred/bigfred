package z21server

import "encoding/binary"

// CentralState bits (Z21 §2.18).
const (
	csEmergencyStop         uint8 = 0x01
	csTrackVoltageOff       uint8 = 0x02
	csShortCircuit          uint8 = 0x04
	csProgrammingModeActive uint8 = 0x20
)

// broadcastFlagSystemState enables LAN_SYSTEMSTATE_DATACHANGED pushes.
const broadcastFlagSystemState uint32 = 0x00000100

// Plausible idle readings for a retail Roco Z21 (15 V PSU, track on).
const (
	emuMainCurrentMA         int16  = 22
	emuProgCurrentMA         int16  = 0
	emuFilteredMainCurrentMA int16  = 20
	emuTemperatureC          int16  = 38
	emuSupplyVoltageMV       uint16 = 15200 // 15.2 V input
	emuVCCVoltageMV          uint16 = 12000 // 12.0 V track
)

// SystemState is the 16-byte payload of LAN_SYSTEMSTATE_DATACHANGED.
type SystemState struct {
	MainCurrent         int16
	ProgCurrent         int16
	FilteredMainCurrent int16
	Temperature         int16
	SupplyVoltage       uint16
	VCCVoltage          uint16
	CentralState        uint8
	CentralStateEx      uint8
	Capabilities        uint8
}

// DefaultSystemState returns emulated retail Z21 telemetry with track power on.
func DefaultSystemState() SystemState {
	return SystemState{
		MainCurrent:         emuMainCurrentMA,
		ProgCurrent:         emuProgCurrentMA,
		FilteredMainCurrent: emuFilteredMainCurrentMA,
		Temperature:         emuTemperatureC,
		SupplyVoltage:       emuSupplyVoltageMV,
		VCCVoltage:          emuVCCVoltageMV,
		CentralState:        0, // track on, no estop/short/prog
		CentralStateEx:      0,
		Capabilities:        capDCC | capLocoCmds | capAccessoryCmds,
	}
}

func (s *Server) effectiveSystemState() SystemState {
	if s.cfg.SystemState != nil {
		return *s.cfg.SystemState
	}
	return DefaultSystemState()
}

func (st SystemState) encode() []byte {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint16(data[0:2], uint16(st.MainCurrent))
	binary.LittleEndian.PutUint16(data[2:4], uint16(st.ProgCurrent))
	binary.LittleEndian.PutUint16(data[4:6], uint16(st.FilteredMainCurrent))
	binary.LittleEndian.PutUint16(data[6:8], uint16(st.Temperature))
	binary.LittleEndian.PutUint16(data[8:10], st.SupplyVoltage)
	binary.LittleEndian.PutUint16(data[10:12], st.VCCVoltage)
	data[12] = st.CentralState
	data[13] = st.CentralStateEx
	data[15] = st.Capabilities
	return data
}

func buildSystemStateReply(st SystemState) []byte {
	return buildReply(HeaderSystemStateData, st.encode())
}

func (s *Server) pushSystemState(client *Client) {
	if s.conn == nil {
		return
	}
	_ = s.writeUDP(&client.Addr, client.Key, buildSystemStateReply(s.effectiveSystemState()))
}

func (s *Server) applyBroadcastFlags(client *Client, flags uint32) {
	s.registry.SetBroadcastFlags(client.Key, flags)
	if flags&broadcastFlagSystemState != 0 {
		s.pushSystemState(client)
	}
}
