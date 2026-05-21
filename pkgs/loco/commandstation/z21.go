package commandstation

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// NewZ21Roco constructor
func NewZ21Roco(netAddr string, netPort uint16) (*Z21Roco, error) {
	roco := Z21Roco{Timeout: time.Second * 10, wasPowerCutOff: false}
	return &roco, roco.connect(fmt.Sprintf("%s:%d", netAddr, netPort))
}

type Z21Roco struct {
	conn           net.Conn
	Timeout        time.Duration
	wasPowerCutOff bool
	// fnStateCache keeps the last known function state bytes per locomotive.
	// Keyed by address; value is 5 bytes covering F0..F31 as in LAN_X_LOCO_INFO (DB4..DB8).
	fnStateCache map[LocoAddr]fnState
	fnStateMu    sync.Mutex
}

// fnState represents function bits F0..F31 for a single loco, as reported
// by LAN_X_LOCO_INFO. The layout follows DB4..DB8.
//
// Bit mapping (per Z21 spec, simplified):
//
//	DB4 (b7..b0): F0..F4 and direction bits (we only care about F0..F4 here)
//	DB5: F5..F12
//	DB6: F13..F20
//	DB7: F21..F28
//	DB8: F29..F31 (not all bits used)
type fnState struct {
	B0_4   byte // DB4
	B5_12  byte // DB5
	B13_20 byte // DB6
	B21_28 byte // DB7
	B29_31 byte // DB8
}

func (z *Z21Roco) connect(netAddr string) error {
	conn, err := net.Dial("udp", netAddr)
	if err != nil {
		return fmt.Errorf("UDP dial error while connecting to Roco Z21: %s", err)
	}
	z.conn = conn
	// initialize cache
	z.fnStateMu.Lock()
	if z.fnStateCache == nil {
		z.fnStateCache = make(map[LocoAddr]fnState)
	}
	z.fnStateMu.Unlock()
	return nil
}

func (Z *Z21Roco) CleanUp() error {
	if Z.wasPowerCutOff {
		logrus.Debug("Restoring power on programming track")
		Z.buildTrackPowerOn()
	}
	return Z.conn.Close()
}

func (Z *Z21Roco) markBuildTrackPowerOff() {
	logrus.Debug("Marking programmng track as to be powered off")
	Z.wasPowerCutOff = true
}

func (z *Z21Roco) buildCVRequest(mode Mode, lcv LocoCV, isWriteRequest bool) ([]byte, error) {
	var err error
	var req []byte

	switch mode {
	case MainTrackMode:
		if isWriteRequest {
			req = z.buildPomWriteByte(lcv)
		} else {
			req = z.buildPomReadPacket(lcv)
		}
	case ProgrammingTrackMode:
		if isWriteRequest {
			req = z.buildProgWritePacket(lcv)
		} else {
			req = z.buildProgReadPacket(lcv.Cv)
		}
	default:
		return []byte{}, errors.New("unrecognized mode")
	}

	return req, err
}

func (z *Z21Roco) WriteCV(mode Mode, lcv LocoCV, options ...ctxOptions) error {
	ctx := RequestContext{timeout: z.Timeout, verify: false, retries: 2, settle: 200}
	applyMethodsToCtx(&ctx, options)

	req, err := z.buildCVRequest(mode, lcv, true)
	if err != nil {
		return fmt.Errorf("cannot build CV request in WriteCV: %s", err.Error())
	}

	// we need to restore the power later on
	if mode == ProgrammingTrackMode {
		defer z.markBuildTrackPowerOff()
	}

	logrus.Debugf("Writing CV: loco=%d, CV%d=%d", lcv.LocoId, lcv.Cv.Num, lcv.Cv.Value)
	if _, writeErr := z.write(req); writeErr != nil {
		return fmt.Errorf("cannot write CV: %s", writeErr.Error())
	}

	if ctx.verify {
		logrus.Debug("Verifying written CV")
		time.Sleep(ctx.settle)
		res, readErr := z.readCVValue(mode, lcv, ctx.timeout, ctx.retries)
		if readErr != nil {
			return fmt.Errorf("cannot verify CV was written: %s", readErr.Error())
		}
		if res.value != byte(lcv.Cv.Value) {
			return fmt.Errorf("cannot write CV, the value differs after a write")
		}
	}

	return nil
}

// ReadCV reads a CV
func (z *Z21Roco) ReadCV(mode Mode, lcv LocoCV, options ...ctxOptions) (int, error) {
	ctx := RequestContext{timeout: z.Timeout, verify: false, retries: 2, settle: 200}
	applyMethodsToCtx(&ctx, options)

	// we need to restore the power later on
	if mode == ProgrammingTrackMode {
		defer z.markBuildTrackPowerOff()
	}

	res, readErr := z.readCVValue(mode, lcv, ctx.timeout, ctx.retries)
	if readErr != nil {
		return 0, fmt.Errorf("cannot read CV: %s", readErr.Error())
	}
	return int(res.value), nil
}

// Sends a function request to the decoder
func (z *Z21Roco) SendFn(mode Mode, addr LocoAddr, num FuncNum, toggle bool) error {
	if mode != MainTrackMode {
		return fmt.Errorf("SendFn: unsupported mode %s", mode)
	}

	fn := int(num)
	if fn < 0 || fn > 31 {
		return fmt.Errorf("SendFn: unsupported function number %d (must be 0-31)", num)
	}

	// Build and send the function command
	req := z.buildSetLocoFunction(addr, fn, toggle)
	logrus.Debugf("req(LAN_X_SET_LOCO_FUNCTION): %v", req)
	if _, err := z.write(req); err != nil {
		return fmt.Errorf("SendFn: cannot write function command: %s", err)
	}

	// Update our cache with the new state
	z.updateFunctionStateCache(addr, fn, toggle)

	return nil
}

// ListFunctions retrieves all active functions for a locomotive and returns their numbers
func (z *Z21Roco) ListFunctions(addr LocoAddr) ([]int, error) {
	// Query the command station using LAN_X_GET_LOCO_INFO
	req := z.buildGetLocoInfo(addr)
	logrus.Debugf("req(LAN_X_GET_LOCO_INFO): %v", req)
	if _, err := z.write(req); err != nil {
		return nil, fmt.Errorf("failed to send LAN_X_GET_LOCO_INFO: %w", err)
	}

	// Wait for response (LAN_X_LOCO_INFO)
	_ = z.conn.SetReadDeadline(time.Now().Add(z.Timeout))
	buf := make([]byte, 1500)
	n, err := z.conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read LAN_X_LOCO_INFO response: %w", err)
	}
	logrus.Debugf("resp(LAN_X_LOCO_INFO): % X", buf[:n])

	// Parse the response
	state, err := z.parseLocoInfo(buf[:n])
	if err != nil {
		return nil, fmt.Errorf("failed to parse LAN_X_LOCO_INFO: %w", err)
	}

	// Cache the state for future reference
	z.fnStateMu.Lock()
	z.fnStateCache[addr] = state
	z.fnStateMu.Unlock()

	// Extract all active functions (F0..F31)
	var activeFunctions []int
	for fnNum := 0; fnNum <= 31; fnNum++ {
		if z.extractFunctionBit(&state, fnNum) {
			activeFunctions = append(activeFunctions, fnNum)
		}
	}

	return activeFunctions, nil
}

type cvResult struct {
	cv     uint16 // 0=CV1 (N+1)
	value  byte
	source string // LAN_X_CV_RESULT/NACK/NACK_SC
}

func (res *cvResult) Error() error {
	switch res.source {
	// ok, we return a correct result
	case "LAN_X_CV_RESULT":
		return nil
	// below are errors returned by Command Station, so the network is okay, but the error is on the protocol side / input data
	case "LAN_X_CV_NACK":
		return fmt.Errorf("missing RailCom acknowledgement (NACK_SC)")
	case "LAN_X_CV_NACK_SC":
		return fmt.Errorf("short circuit (LAN_X_CV_NACK_SC)")
	}
	return fmt.Errorf("unknown error (%s)", res.source)
}

func (z *Z21Roco) parseCVResponse(pkt []byte) (cvResult, bool) {
	if len(pkt) < 6 {
		return cvResult{}, false
	}
	dataLen := binary.LittleEndian.Uint16(pkt[0:2])
	header := binary.LittleEndian.Uint16(pkt[2:4])
	if header != 0x0040 || int(dataLen) != len(pkt) {
		return cvResult{}, false
	}

	// RESULT: 64 14 CV_MSB CV_LSB Value XOR
	if len(pkt) >= 10 && pkt[4] == 0x64 && pkt[5] == 0x14 {
		return cvResult{
			cv:     (uint16(pkt[6]) << 8) | uint16(pkt[7]),
			value:  pkt[8],
			source: "LAN_X_CV_RESULT",
		}, true
	}
	// NACKs
	if pkt[4] == 0x61 && pkt[5] == 0x13 {
		return cvResult{source: "LAN_X_CV_NACK"}, true
	}
	if pkt[4] == 0x61 && pkt[5] == 0x12 {
		return cvResult{source: "LAN_X_CV_NACK_SC"}, true
	}
	return cvResult{}, false
}

// Sends and waits for LAN_X_CV_* (read or write-result)
func (z *Z21Roco) sendAndAwait(req []byte, timeout time.Duration) (cvResult, error) {
	logrus.Debugf("z21.sendAndAwait: % X", req)
	if _, err := z.write(req); err != nil {
		return cvResult{}, err
	}
	_ = z.conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 1500)
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		n, err := z.conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return cvResult{}, errors.New("response timeout")
			}
			return cvResult{}, err
		}
		if res, ok := z.parseCVResponse(buf[:n]); ok {
			return res, nil
		}
	}
	return cvResult{}, errors.New("no response or unrecognized response")
}

// readCVValue is reading the POM/PROG CV response
func (z *Z21Roco) readCVValue(mode Mode, lcv LocoCV, timeout time.Duration, retries uint8) (cvResult, error) {
	req, reqErr := z.buildCVRequest(mode, lcv, false)
	if reqErr != nil {
		return cvResult{}, fmt.Errorf("cannot build CV request: %s", reqErr)
	}

	var lastErr error
	for i := 0; i <= int(retries); i++ {
		logrus.Debugf("Try [%d/%d]", i, retries)
		res, err := z.sendAndAwait(req, timeout)
		if err == nil {
			if responseErr := res.Error(); responseErr != nil {
				lastErr = fmt.Errorf("cannot read CV: %s", responseErr.Error())
				err = lastErr
				continue
			}

			return res, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	return cvResult{}, lastErr
}

// parseLocoInfo parses LAN_X_LOCO_INFO response (0xEF)
func (z *Z21Roco) parseLocoInfo(pkt []byte) (fnState, error) {
	if len(pkt) < 7 {
		return fnState{}, fmt.Errorf("packet too short: %d bytes", len(pkt))
	}

	dataLen := binary.LittleEndian.Uint16(pkt[0:2])
	header := binary.LittleEndian.Uint16(pkt[2:4])

	if header != 0x0040 || int(dataLen) != len(pkt) {
		return fnState{}, fmt.Errorf("invalid header or length")
	}

	if pkt[4] != 0xEF {
		return fnState{}, fmt.Errorf("not a LAN_X_LOCO_INFO packet (X-Header: 0x%02X)", pkt[4])
	}

	// LAN_X_LOCO_INFO structure:
	// Byte 0-1: DataLen (little endian)
	// Byte 2-3: Header 0x0040 (little endian)
	// Byte 4: X-Header 0xEF
	// Byte 5: DB0 (address MSB)
	// Byte 6: DB1 (address LSB)
	// Byte 7: DB2 (speed/direction info)
	// Byte 8: DB3 (speed value)
	// Byte 9: DB4 (F0-F4 with direction)
	// Byte 10: DB5 (F5-F12)
	// Byte 11: DB6 (F13-F20) [optional]
	// Byte 12: DB7 (F21-F28) [optional]
	// Byte 13: DB8 (F29-F31) [optional, from FW 1.42+]
	// Last byte: XOR

	var state fnState

	// DB4 (F0-F4) is at byte 9
	if len(pkt) > 9 {
		state.B0_4 = pkt[9]
	}

	// DB5 (F5-F12) is at byte 10
	if len(pkt) > 10 {
		state.B5_12 = pkt[10]
	}

	// DB6 (F13-F20) is at byte 11
	if len(pkt) > 11 {
		state.B13_20 = pkt[11]
	}

	// DB7 (F21-F28) is at byte 12
	if len(pkt) > 12 {
		state.B21_28 = pkt[12]
	}

	// DB8 (F29-F31) is at byte 13
	if len(pkt) > 13 {
		state.B29_31 = pkt[13]
	}

	return state, nil
}

// extractFunctionBit extracts the state of a specific function from fnState
func (z *Z21Roco) extractFunctionBit(state *fnState, fnNum int) bool {
	switch {
	case fnNum == 0:
		// F0 is bit 4 in DB4
		return (state.B0_4 & 0x10) != 0
	case fnNum >= 1 && fnNum <= 4:
		// F1-F4 are bits 0-3 in DB4
		return (state.B0_4 & (1 << (fnNum - 1))) != 0
	case fnNum >= 5 && fnNum <= 12:
		// F5-F12 are bits 0-7 in DB5
		return (state.B5_12 & (1 << (fnNum - 5))) != 0
	case fnNum >= 13 && fnNum <= 20:
		// F13-F20 are bits 0-7 in DB6
		return (state.B13_20 & (1 << (fnNum - 13))) != 0
	case fnNum >= 21 && fnNum <= 28:
		// F21-F28 are bits 0-7 in DB7
		return (state.B21_28 & (1 << (fnNum - 21))) != 0
	case fnNum >= 29 && fnNum <= 31:
		// F29-F31 are bits 0-2 in DB8
		return (state.B29_31 & (1 << (fnNum - 29))) != 0
	default:
		return false
	}
}

// updateFunctionStateCache updates the cached function state for a locomotive
func (z *Z21Roco) updateFunctionStateCache(addr LocoAddr, fnNum int, on bool) {
	z.fnStateMu.Lock()
	defer z.fnStateMu.Unlock()

	state, ok := z.fnStateCache[addr]
	if !ok {
		// Initialize empty state if not present
		state = fnState{}
	}

	// Update the appropriate bit
	switch {
	case fnNum == 0:
		if on {
			state.B0_4 |= 0x10
		} else {
			state.B0_4 &^= 0x10
		}
	case fnNum >= 1 && fnNum <= 4:
		mask := byte(1 << (fnNum - 1))
		if on {
			state.B0_4 |= mask
		} else {
			state.B0_4 &^= mask
		}
	case fnNum >= 5 && fnNum <= 12:
		mask := byte(1 << (fnNum - 5))
		if on {
			state.B5_12 |= mask
		} else {
			state.B5_12 &^= mask
		}
	case fnNum >= 13 && fnNum <= 20:
		mask := byte(1 << (fnNum - 13))
		if on {
			state.B13_20 |= mask
		} else {
			state.B13_20 &^= mask
		}
	case fnNum >= 21 && fnNum <= 28:
		mask := byte(1 << (fnNum - 21))
		if on {
			state.B21_28 |= mask
		} else {
			state.B21_28 &^= mask
		}
	case fnNum >= 29 && fnNum <= 31:
		mask := byte(1 << (fnNum - 29))
		if on {
			state.B29_31 |= mask
		} else {
			state.B29_31 &^= mask
		}
	}

	z.fnStateCache[addr] = state
}

// SetSpeed sets the speed and direction of a locomotive
// speed: 0=stop, 1=emergency stop, 2+ for actual speed (max depends on speedSteps)
// forward: true for forward, false for reverse
// speedSteps: 14, 28, or 128 (will be converted to 0, 2, or 4 for the protocol)
func (z *Z21Roco) SetSpeed(addr LocoAddr, speed uint8, forward bool, speedSteps uint8) error {
	// Convert speedSteps to protocol value
	var speedStepsProto uint8
	switch speedSteps {
	case 14:
		speedStepsProto = 0
	case 28:
		speedStepsProto = 2
	case 128:
		speedStepsProto = 4
	default:
		return fmt.Errorf("invalid speed steps: %d (must be 14, 28, or 128)", speedSteps)
	}

	// Build and send the speed command
	req := z.buildSetLocoSpeed(addr, speed, forward, speedStepsProto)
	logrus.Debugf("req(LAN_X_SET_LOCO_DRIVE): % X", req)
	if _, err := z.write(req); err != nil {
		return fmt.Errorf("SetSpeed: cannot write speed command: %w", err)
	}

	return nil
}

// GetSpeed retrieves the current speed and direction of a locomotive
// Returns: speed (0-127), forward (true for forward, false for reverse), error
func (z *Z21Roco) GetSpeed(addr LocoAddr) (uint8, bool, error) {
	// Query the command station using LAN_X_GET_LOCO_INFO
	req := z.buildGetLocoInfo(addr)
	logrus.Debugf("req(LAN_X_GET_LOCO_INFO): %v", req)
	if _, err := z.write(req); err != nil {
		return 0, false, fmt.Errorf("failed to send LAN_X_GET_LOCO_INFO: %w", err)
	}

	// Wait for response (LAN_X_LOCO_INFO)
	_ = z.conn.SetReadDeadline(time.Now().Add(z.Timeout))
	buf := make([]byte, 1500)
	n, err := z.conn.Read(buf)
	if err != nil {
		return 0, false, fmt.Errorf("failed to read LAN_X_LOCO_INFO response: %w", err)
	}
	logrus.Debugf("resp(LAN_X_LOCO_INFO): % X", buf[:n])

	// Parse the response to extract speed and direction
	// LAN_X_LOCO_INFO structure:
	// Byte 0-1: DataLen (little endian)
	// Byte 2-3: Header 0x0040 (little endian)
	// Byte 4: X-Header 0xEF
	// Byte 5: DB0 (address MSB)
	// Byte 6: DB1 (address LSB)
	// Byte 7: DB2 (speed/direction info) - bit 7: direction (0=forward, 1=reverse), bits 5-0: speed steps
	// Byte 8: DB3 (speed value)

	if len(buf) < 9 {
		return 0, false, fmt.Errorf("response too short")
	}

	dataLen := binary.LittleEndian.Uint16(buf[0:2])
	header := binary.LittleEndian.Uint16(buf[2:4])

	if header != 0x0040 || int(dataLen) != n {
		return 0, false, fmt.Errorf("invalid header or length in response")
	}

	if buf[4] != 0xEF {
		return 0, false, fmt.Errorf("not a LAN_X_LOCO_INFO packet (X-Header: 0x%02X)", buf[4])
	}

	// Extract direction from DB2 (byte 7): bit 7 indicates direction
	// bit 7: 0 = forward, 1 = reverse
	directionBit := (buf[7] & 0x80) != 0
	forward := !directionBit // invert because 1 means reverse

	// Extract speed from DB3 (byte 8)
	speed := buf[8]

	return speed, forward, nil
}
