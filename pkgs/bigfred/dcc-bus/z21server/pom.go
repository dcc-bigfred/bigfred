package z21server

import (
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const (
	pomReadByteOption = 0xE4
)

func virtualCVKey(loco uint16, cvWire int) uint32 {
	return uint32(loco)<<16 | uint32(cvWire&0xffff)
}

func (c *Client) setVirtualCV(loco uint16, cvWire int, value byte) {
	if c.virtualCV == nil {
		c.virtualCV = make(map[uint32]byte)
	}
	c.virtualCV[virtualCVKey(loco, cvWire)] = value
}

func (c *Client) getVirtualCV(loco uint16, cvWire int) (byte, bool) {
	if c.virtualCV == nil {
		return 0, false
	}
	v, ok := c.virtualCV[virtualCVKey(loco, cvWire)]
	return v, ok
}

func isPOMWriteByte(pkt []byte) bool {
	_, cvWire, _, ok := parsePOMWriteByte(pkt)
	return ok && cvWire >= 0
}

func isPOMReadByte(pkt []byte) bool {
	_, _, ok := parsePOMReadByte(pkt)
	return ok
}

func isPairingCVWire(cvWire int) bool {
	return cvWire == cvWireCV3 || cvWire == cvWireCV4
}

// parsePOMReadByte extracts loco address and CV wire index from LAN_X_CV_POM_READ_BYTE.
func parsePOMReadByte(pkt []byte) (locoAddr uint16, cvWire int, ok bool) {
	if len(pkt) < 12 {
		return 0, 0, false
	}
	_, header, okHdr := packetHeader(pkt)
	if !okHdr || header != HeaderXBus || pkt[4] != 0xE6 || pkt[5] != 0x30 {
		return 0, 0, false
	}
	db3 := pkt[8]
	if (db3 & 0xFC) != pomReadByteOption {
		return 0, 0, false
	}
	locoAddr, ok = parseLocoAddr(pkt, 6)
	if !ok {
		return 0, 0, false
	}
	cvWire = int((db3&0x03)<<8) | int(pkt[9])
	return locoAddr, cvWire, true
}

func buildCVResultReply(cvWire int, value byte) []byte {
	cvMSB := byte((cvWire >> 8) & 0x03)
	cvLSB := byte(cvWire & 0xFF)
	x := []byte{0x64, 0x14, cvMSB, cvLSB, value}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func buildCVNackReply() []byte {
	x := []byte{0x61, 0x13}
	x = append(x, xorSum(x))
	return buildXBusReply(x)
}

func cvWireToNum(cvWire int) commandstation.CVNum {
	return commandstation.CVNum(cvWire + 1)
}
