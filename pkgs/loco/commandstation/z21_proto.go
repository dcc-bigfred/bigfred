package commandstation

import (
	"encoding/binary"

	"github.com/sirupsen/logrus"
)

//
// Context: This file is containing methods to communicate with a DCC device using a Z21 protocol
//

// Read: LAN_X_CV_POM_READ_BYTE (E6 30 … option 0xE4)
func (z *Z21Roco) buildPomReadPacket(lcv LocoCV) []byte {
	const dataLen, header = 0x000C, 0x0040
	cvWire := lcv.Cv.Translate()

	adrMSB := byte((lcv.LocoId >> 8) & 0x3F)
	if lcv.LocoId >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(lcv.LocoId & 0xFF)
	db3 := byte(0xE4 | byte((cvWire>>8)&0x03)) // 111001MM
	db4 := byte(cvWire & 0xFF)
	x := []byte{0xE6, 0x30, adrMSB, adrLSB, db3, db4, 0x00}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// Write BYTE: LAN_X_CV_POM_WRITE_BYTE (E6 30 … option 0xEC)
func (z *Z21Roco) buildPomWriteByte(lcv LocoCV) []byte {
	const dataLen, header = 0x000C, 0x0040
	addr := lcv.LocoId
	cvWire := lcv.Cv.Translate()
	value := byte(lcv.Cv.Value)

	adrMSB := byte((addr >> 8) & 0x3F)
	if addr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(addr & 0xFF)
	db3 := byte(0xEC | byte((cvWire>>8)&0x03)) // 111011MM
	db4 := byte(cvWire & 0xFF)
	x := []byte{0xE6, 0x30, adrMSB, adrLSB, db3, db4, value}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// ===== PROG (Programming Track / Direct Mode) =====
// Read: LAN_X_CV_READ (23 11)
func (z *Z21Roco) buildProgReadPacket(cv CV) []byte {
	const dataLen, header = 0x000B, 0x0040
	cvWire := cv.Translate()

	x := []byte{0x23, 0x11, byte(cvWire >> 8), byte(cvWire & 0xFF)}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// Write: LAN_X_CV_WRITE (24 12)
func (z *Z21Roco) buildProgWritePacket(lcv LocoCV) []byte {
	const dataLen, header = 0x000A, 0x0040
	cvWire := lcv.Cv.Translate()
	value := byte(lcv.Cv.Value)

	x := []byte{0x24, 0x12, byte(cvWire >> 8), byte(cvWire & 0xFF), value}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// Track power ON (get back from programming mode)
func (z *Z21Roco) buildTrackPowerOn() []byte {
	const dataLen, header = 0x0007, 0x0040
	x := []byte{0x21, 0x81}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// buildGetLocoInfo builds LAN_X_GET_LOCO_INFO command (0xE3 0xF0)
func (z *Z21Roco) buildGetLocoInfo(addr LocoAddr) []byte {
	const dataLen, header = 0x0009, 0x0040

	adrMSB := byte((addr >> 8) & 0x3F)
	if addr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(addr & 0xFF)

	x := []byte{0xE3, 0xF0, adrMSB, adrLSB}
	x = append(x, xorSum(x))

	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// buildSetLocoFunction builds LAN_X_SET_LOCO_FUNCTION command (0xE4 0xF8)
func (z *Z21Roco) buildSetLocoFunction(addr LocoAddr, fnNum int, on bool) []byte {
	const dataLen, header = 0x000A, 0x0040

	adrMSB := byte((addr >> 8) & 0x3F)
	if addr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(addr & 0xFF)

	// DB3: TT FFFFFF where TT = type (00=off, 01=on, 10=toggle), FFFFFF = function number
	var typeBits byte
	if on {
		typeBits = 0x40 // 01 << 6 = turn on
	} else {
		typeBits = 0x00 // 00 << 6 = turn off
	}
	db3 := typeBits | byte(fnNum&0x3F)

	x := []byte{0xE4, 0xF8, adrMSB, adrLSB, db3}
	x = append(x, xorSum(x))

	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// buildSetLocoSpeed builds LAN_X_SET_LOCO_DRIVE command (0xE4 0x1S)
// speedSteps: 0=14 steps, 2=28 steps, 4=128 steps
// speed: 0=stop, 1=emergency stop, 2-127 (for 128 steps) actual speed
// forward: true for forward direction, false for reverse
func (z *Z21Roco) buildSetLocoSpeed(addr LocoAddr, speed uint8, forward bool, speedSteps uint8) []byte {
	const dataLen, header = 0x000A, 0x0040

	adrMSB := byte((addr >> 8) & 0x3F)
	if addr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(addr & 0xFF)

	// DB0: 0x1S where S = speed steps (0=14, 2=28, 4=128)
	db0 := byte(0x10 | (speedSteps & 0x0F))

	// DB3: RVVVVVVV where R = direction (1=forward) and V = speed
	var db3 byte
	if forward {
		db3 = 0x80 // Set direction bit
	}

	// Speed encoding depends on speed steps
	switch speedSteps {
	case 0: // 14 speed steps
		// For DCC 14: speed 0=stop, 1=e-stop, 2-15 are steps 1-14
		if speed > 15 {
			speed = 15
		}
		db3 |= (speed & 0x0F)
	case 2: // 28 speed steps
		// For DCC 28: more complex encoding with bit 5
		if speed > 28 {
			speed = 28
		}
		if speed > 0 {
			// Map 1-28 to the encoding shown in the spec
			speedBits := byte((speed + 3) / 2) // bits 0-3
			speedBit5 := byte((speed + 3) % 2) // bit 5
			db3 |= (speedBit5 << 4) | (speedBits & 0x0F)
		}
	case 4: // 128 speed steps (default)
		// For DCC 128: speed 0=stop, 1=e-stop, 2-127 are steps 1-126
		if speed > 127 {
			speed = 127
		}
		db3 |= (speed & 0x7F)
	default:
		// Default to 128 speed steps
		db3 |= (speed & 0x7F)
	}

	x := []byte{0xE4, db0, adrMSB, adrLSB, db3}
	x = append(x, xorSum(x))

	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

func (z *Z21Roco) write(b []byte) (n int, err error) {
	logrus.Debugf("write: % X", b)
	return z.conn.Write(b)
}
