package commandstation

import (
	"encoding/hex"
	"fmt"
)

// LocoNet checksum:
// The XOR of all bytes including the checksum byte must equal 0xFF.
func lnChecksumOK(pkt []byte) bool {
	if len(pkt) < 2 {
		return false
	}
	var x byte
	for _, b := range pkt {
		x ^= b
	}
	return x == 0xFF
}

func lnAppendChecksum(msg []byte) []byte {
	var x byte
	for _, b := range msg {
		x ^= b
	}
	// want x ^ chk == 0xFF => chk == x ^ 0xFF
	return append(msg, x^0xFF)
}

func lnMsgLen(opcode byte, buf []byte) (int, bool) {
	// Length encoding is in bits 5..6:
	// 00 => 2 bytes, 01 => 4 bytes, 10 => 6 bytes, 11 => variable length, second byte is total length.
	switch (opcode >> 5) & 0x03 {
	case 0:
		return 2, true
	case 1:
		return 4, true
	case 2:
		return 6, true
	default:
		// variable
		if len(buf) < 2 {
			return 0, false
		}
		l := int(buf[1])
		if l < 2 {
			return 0, false
		}
		return l, true
	}
}

type lnPacket []byte

func (p lnPacket) String() string {
	if len(p) == 0 {
		return "<empty>"
	}
	return hex.EncodeToString([]byte(p))
}

// lnStreamParser incrementally reconstructs packets from a byte stream.
type lnStreamParser struct {
	cur []byte
}

func (p *lnStreamParser) PushByte(b byte) (pkt []byte, ok bool) {
	// Message starts at byte with opcode bit7 = 1.
	if len(p.cur) == 0 {
		if (b & 0x80) == 0 {
			return nil, false
		}
		p.cur = append(p.cur, b)
		return nil, false
	}

	p.cur = append(p.cur, b)

	want, known := lnMsgLen(p.cur[0], p.cur)
	if !known || want == 0 {
		return nil, false
	}
	if len(p.cur) < want {
		return nil, false
	}

	// If we have more than want (should not happen), resync on next opcode.
	if len(p.cur) > want {
		// find next opcode byte within p.cur[1:]
		next := -1
		for i := 1; i < len(p.cur); i++ {
			if (p.cur[i] & 0x80) != 0 {
				next = i
				break
			}
		}
		if next == -1 {
			p.cur = p.cur[:0]
			return nil, false
		}
		p.cur = append([]byte{}, p.cur[next:]...)
		return nil, false
	}

	pkt = append([]byte{}, p.cur...)
	p.cur = p.cur[:0]
	return pkt, true
}

const (
	lnOPC_LOCO_ADR   = 0xBF
	lnOPC_RQ_SL_DATA = 0xBB
	lnOPC_LOCO_SPD   = 0xA0
	lnOPC_LOCO_DIRF  = 0xA1
	lnOPC_LOCO_SND   = 0xA2
	lnOPC_SL_RD_DATA = 0xE7
	lnOPC_LONG_ACK   = 0xB4
	lnOPC_BUSY       = 0x81
	lnOPC_GPOFF      = 0x82
	lnOPC_GPON       = 0x83
	lnOPC_IDLE       = 0x85
)

func lnBuildLocoAdr(addr LocoAddr) []byte {
	lo := byte(addr & 0x7F)
	hi := byte((addr >> 7) & 0x7F)
	return lnAppendChecksum([]byte{lnOPC_LOCO_ADR, 0x00, lo, hi})
}

func lnBuildRqSlotData(slot byte) []byte {
	return lnAppendChecksum([]byte{lnOPC_RQ_SL_DATA, slot, 0x00})
}

func lnBuildSetSpeed(slot byte, speed byte) []byte {
	return lnAppendChecksum([]byte{lnOPC_LOCO_SPD, slot, speed})
}

func lnBuildSetDirF(slot byte, dirf byte) []byte {
	return lnAppendChecksum([]byte{lnOPC_LOCO_DIRF, slot, dirf})
}

func lnBuildSetSnd(slot byte, snd byte) []byte {
	return lnAppendChecksum([]byte{lnOPC_LOCO_SND, slot, snd})
}

type lnSlotData struct {
	Slot  byte
	Addr  LocoAddr
	Speed byte
	DirF  byte
	Snd   byte
}

func parseLnSlotData(pkt []byte) (lnSlotData, bool) {
	// Expect: E7 0E <slot> <stat1> <adr_lo> <spd> <dirf> <trk> <stat2> <adr_hi> <snd> <id1> <id2> <chk>
	if len(pkt) < 14 || pkt[0] != lnOPC_SL_RD_DATA {
		return lnSlotData{}, false
	}
	if pkt[1] != 0x0E {
		// not the classic slot read length; still allow parsing if long enough
	}
	if !lnChecksumOK(pkt) {
		return lnSlotData{}, false
	}
	slot := pkt[2]
	adrLo := pkt[4]
	adrHi := pkt[9]
	addr := LocoAddr(adrLo&0x7F) | (LocoAddr(adrHi&0x7F) << 7)
	return lnSlotData{
		Slot:  slot,
		Addr:  addr,
		Speed: pkt[5],
		DirF:  pkt[6],
		Snd:   pkt[10],
	}, true
}

func scaleToLnSpeed(speed uint8, speedSteps uint8) (byte, error) {
	// LocoNet slot speed semantics:
	// 0x00 stop, 0x01 e-stop, 0x02..0x7F = increasing speed.
	if speedSteps != 14 && speedSteps != 28 && speedSteps != 128 {
		return 0, fmt.Errorf("invalid speed steps: %d (must be 14, 28, or 128)", speedSteps)
	}
	if speed == 0 || speed == 1 {
		return byte(speed), nil
	}

	// Map user speed (2..max) to 2..127 linearly.
	var max uint8
	switch speedSteps {
	case 14:
		max = 15
	case 28:
		max = 28
	case 128:
		max = 127
	}
	if speed > max {
		speed = max
	}
	// scale 2..max -> 2..127
	in := int(speed - 2)
	inMax := int(max - 2)
	if inMax <= 0 {
		return 2, nil
	}
	out := 2 + (in*int(127-2))/inMax
	if out < 2 {
		out = 2
	}
	if out > 127 {
		out = 127
	}
	return byte(out), nil
}
