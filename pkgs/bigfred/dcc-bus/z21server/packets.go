package z21server

import (
	"encoding/binary"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

const (
	pomWriteByteOption = 0xEC
	cvWireCV3          = 2
	cvWireCV4          = 3
)

// parsePOMWriteByte extracts loco address, CV wire index and value from LAN_X_CV_POM_WRITE_BYTE.
func parsePOMWriteByte(pkt []byte) (locoAddr uint16, cvWire int, value int, ok bool) {
	if len(pkt) < 12 {
		return 0, 0, 0, false
	}
	_, header, okHdr := packetHeader(pkt)
	if !okHdr || header != HeaderXBus || pkt[4] != 0xE6 || pkt[5] != 0x30 {
		return 0, 0, 0, false
	}
	db3 := pkt[8]
	if (db3 & 0xFC) != pomWriteByteOption {
		return 0, 0, 0, false
	}
	locoAddr, ok = parseLocoAddr(pkt, 6)
	if !ok {
		return 0, 0, 0, false
	}
	cvWire = int(db3&0x03)<<8 | int(pkt[9])
	value = int(pkt[10])
	return locoAddr, cvWire, value, true
}

func buildPOMWriteByte(locoAddr uint16, cvWire int, value byte) []byte {
	adrMSB := byte((locoAddr >> 8) & 0x3F)
	if locoAddr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(locoAddr & 0xFF)
	db3 := byte(pomWriteByteOption | byte((cvWire>>8)&0x03))
	db4 := byte(cvWire & 0xFF)
	x := []byte{0xE6, 0x30, adrMSB, adrLSB, db3, db4, value}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 4+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, uint16(4+len(x)))
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, HeaderXBus)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

func parseLocoAddr(pkt []byte, offset int) (uint16, bool) {
	if len(pkt) < offset+2 {
		return 0, false
	}
	return uint16(pkt[offset]&0x3F)<<8 | uint16(pkt[offset+1]), true
}

func parseSetLocoDrive(pkt []byte) (addr uint16, speed uint8, forward bool, ok bool) {
	if len(pkt) < 10 {
		return 0, 0, false, false
	}
	_, header, okHdr := packetHeader(pkt)
	if !okHdr || header != HeaderXBus || pkt[4] != 0xE4 || (pkt[5]&0xF0) != 0x10 {
		return 0, 0, false, false
	}
	addr, ok = parseLocoAddr(pkt, 6)
	if !ok {
		return 0, 0, false, false
	}
	speedStepsProto := pkt[5] & 0x0F
	speed, forward = decodeDriveDB3(speedStepsProto, pkt[8])
	return addr, speed, forward, true
}

// funcSwitchType is the DB3 switch-type field of LAN_X_SET_LOCO_FUNCTION
// (Z21 spec 4.3.1): 00 = off, 01 = on, 10 = toggle. 11 is reserved.
type funcSwitchType uint8

const (
	funcSwitchOff    funcSwitchType = 0
	funcSwitchOn     funcSwitchType = 1
	funcSwitchToggle funcSwitchType = 2
)

func parseSetLocoFunction(pkt []byte) (addr uint16, fn int, sw funcSwitchType, ok bool) {
	if len(pkt) < 10 {
		return 0, 0, 0, false
	}
	_, header, okHdr := packetHeader(pkt)
	if !okHdr || header != HeaderXBus || pkt[4] != 0xE4 || pkt[5] != 0xF8 {
		return 0, 0, 0, false
	}
	addr, ok = parseLocoAddr(pkt, 6)
	if !ok {
		return 0, 0, 0, false
	}
	sw = funcSwitchType(pkt[8] >> 6)
	if sw > funcSwitchToggle {
		return 0, 0, 0, false
	}
	return addr, int(pkt[8] & 0x3F), sw, true
}

func parseSetLocoFunctionGroup(pkt []byte) (addr uint16, updates []struct {
	fn int
	on bool
}, ok bool) {
	if len(pkt) < 10 {
		return 0, nil, false
	}
	_, header, okHdr := packetHeader(pkt)
	if !okHdr || header != HeaderXBus || pkt[4] != 0xE4 {
		return 0, nil, false
	}
	group := pkt[5]
	fnMap, okGroup := locoFunctionGroupMap[group]
	if !okGroup {
		return 0, nil, false
	}
	addr, ok = parseLocoAddr(pkt, 6)
	if !ok {
		return 0, nil, false
	}
	fnByte := pkt[8]
	for bit, fn := range fnMap {
		if fn < 0 {
			continue
		}
		updates = append(updates, struct {
			fn int
			on bool
		}{fn: fn, on: fnByte&(1<<uint(bit)) != 0})
	}
	return addr, updates, true
}

// locoFunctionGroupMap maps group id → bit index → function number (-1 = unused).
var locoFunctionGroupMap = map[byte][8]int{
	0x20: {1, 2, 3, 4, 0, -1, -1, -1},
	0x21: {5, 6, 7, 8, -1, -1, -1, -1},
	0x22: {9, 10, 11, 12, -1, -1, -1, -1},
	0x23: {13, 14, 15, 16, 17, 18, 19, 20},
	0x28: {21, 22, 23, 24, 25, 26, 27, 28},
	0x29: {29, 30, 31, -1, -1, -1, -1, -1},
}

func parseGetLocoInfo(pkt []byte) (addr uint16, ok bool) {
	if len(pkt) < 9 {
		return 0, false
	}
	_, header, okHdr := packetHeader(pkt)
	if !okHdr || header != HeaderXBus || pkt[4] != 0xE3 || pkt[5] != 0xF0 {
		return 0, false
	}
	return parseLocoAddr(pkt, 6)
}

func decodeDriveDB3(speedStepsProto, db3 byte) (speed uint8, forward bool) {
	forward = db3&0x80 != 0
	v := db3 & 0x7F
	switch speedStepsProto {
	case 0:
		return v & 0x0F, forward
	case 2:
		speedBits := v & 0x0F
		speedBit5 := (v >> 4) & 0x01
		raw := int(speedBits)*2 + int(speedBit5)
		switch {
		case raw <= 1:
			return 0, forward
		case raw <= 3:
			return 1, forward
		default:
			return uint8(raw - 3), forward
		}
	default:
		return v, forward
	}
}

func buildLocoInfoReply(addr uint16, snap contract.LocoStateWire, speedSteps uint) []byte {
	db2 := locoInfoDB2(speedSteps)
	db3 := encodeInfoDB3(snap.Speed, snap.Forward, speedSteps)
	db4, db5, db6, db7, db8 := encodeFunctionBytes(snap.Functions)

	adrMSB := byte((addr >> 8) & 0x3F)
	if addr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(addr & 0xFF)
	x := []byte{0xEF, adrMSB, adrLSB, db2, db3, db4, db5, db6, db7, db8}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func locoInfoDB2(speedSteps uint) byte {
	switch speedSteps {
	case 14:
		return 0
	case 28:
		return 2
	default:
		return 4
	}
}

func encodeInfoDB3(speed uint8, forward bool, speedSteps uint) byte {
	var proto byte
	switch speedSteps {
	case 14:
		proto = 0
	case 28:
		proto = 2
	default:
		proto = 3
	}
	return encodeDriveDB3(speed, forward, proto)
}

func encodeDriveDB3(speed uint8, forward bool, speedStepsProto byte) byte {
	var db3 byte
	if forward {
		db3 = 0x80
	}
	switch speed {
	case 0:
		return db3
	case 1:
		return db3 | 0x01
	}
	switch speedStepsProto {
	case 0:
		return db3 | (speed & 0x0F)
	case 2:
		if speed > 28 {
			speed = 28
		}
		speedBits := byte((speed + 3) / 2)
		speedBit5 := byte((speed + 3) % 2)
		return db3 | (speedBit5 << 4) | (speedBits & 0x0F)
	default:
		if speed > 127 {
			speed = 127
		}
		return db3 | (speed & 0x7F)
	}
}

func encodeFunctionBytes(functions []bool) (b0, b1, b2, b3, b4 byte) {
	set := func(fn int) {
		if fn >= len(functions) || !functions[fn] {
			return
		}
		switch {
		case fn == 0:
			b0 |= 0x10
		case fn <= 4:
			b0 |= 1 << uint(fn-1)
		case fn <= 12:
			b1 |= 1 << uint(fn-5)
		case fn <= 20:
			b2 |= 1 << uint(fn-13)
		case fn <= 28:
			b3 |= 1 << uint(fn-21)
		case fn <= 31:
			b4 |= 1 << uint(fn-29)
		}
	}
	for fn := 0; fn < 32; fn++ {
		set(fn)
	}
	return b0, b1, b2, b3, b4
}

func buildXBusReply(x []byte) []byte {
	dataLen := uint16(4 + len(x))
	out := make([]byte, 4+len(x))
	binary.LittleEndian.PutUint16(out[0:2], dataLen)
	binary.LittleEndian.PutUint16(out[2:4], HeaderXBus)
	copy(out[4:], x)
	return out
}

func xorSum(x []byte) byte {
	var v byte
	for _, b := range x {
		v ^= b
	}
	return v
}
