package withrottle

import (
	"strconv"
	"strings"
)

const (
	propSep    = "<;>"
	entrySep   = "]\\["
	segmentSep = "}|{"
)

// splitProperty splits line on sep into non-empty parts.
func splitProperty(line, sep string) []string {
	if line == "" {
		return nil
	}
	raw := strings.Split(line, sep)
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseLocoKey parses Snnn or Lnnn into a DCC address.
func parseLocoKey(s string) (addr uint16, isLong bool, ok bool) {
	if len(s) < 2 {
		return 0, false, false
	}
	switch s[0] {
	case 'S', 's':
		n, err := strconv.ParseUint(s[1:], 10, 16)
		if err != nil || n == 0 || n > 127 {
			return 0, false, false
		}
		return uint16(n), false, true
	case 'L', 'l':
		n, err := strconv.ParseUint(s[1:], 10, 16)
		if err != nil || n < 128 || n > 10239 {
			return 0, false, false
		}
		return uint16(n), true, true
	default:
		return 0, false, false
	}
}

// MOp is the operation letter in an M command (M«id»«op»…).
type MOp byte

const (
	MOpAdd    MOp = '+'
	MOpRemove MOp = '-'
	MOpSteal  MOp = 'S'
	MOpAction MOp = 'A'
	MOpLabels MOp = 'L'
)

// MCommand is a parsed MultiThrottle line.
type MCommand struct {
	ThrottleID byte
	Op         MOp
	LocoKey    string
	Properties []string
}

// parseMAction parses M«id»«op»«locoKey»<;>… lines.
func parseMAction(line string) (MCommand, bool) {
	if len(line) < 4 || line[0] != 'M' {
		return MCommand{}, false
	}
	id := line[1]
	op := line[2]
	if op != '+' && op != '-' && op != 'S' && op != 'A' && op != 'L' {
		return MCommand{}, false
	}
	rest := line[3:]
	parts := splitProperty(rest, propSep)
	if len(parts) == 0 {
		return MCommand{}, false
	}
	cmd := MCommand{
		ThrottleID: id,
		Op:         MOp(op),
		LocoKey:    parts[0],
	}
	if len(parts) > 1 {
		cmd.Properties = parts[1:]
	}
	return cmd, true
}

// parseFunctionAction extracts function number and on-state from F/f payloads.
func parseFunctionAction(prop string) (fn int, on bool, force bool, ok bool) {
	if len(prop) < 2 {
		return 0, false, false, false
	}
	switch prop[0] {
	case 'F':
		force = false
	case 'f':
		force = true
	default:
		return 0, false, false, false
	}
	if prop[1] != '0' && prop[1] != '1' {
		return 0, false, false, false
	}
	on = prop[1] == '1'
	n, err := strconv.Atoi(prop[2:])
	if err != nil || n < 0 || n > maxWiThrottleFunction {
		return 0, false, false, false
	}
	return n, on, force, true
}

// locoKeyForAddr formats S or L wire key for a DCC address.
func locoKeyForAddr(addr uint16) string {
	if addr >= 128 {
		return "L" + strconv.Itoa(int(addr))
	}
	return "S" + strconv.Itoa(int(addr))
}

// addrFromLocoKey resolves * or S/L key to addresses on a throttle instance.
func addrFromLocoKey(key string, tw *throttleWire) []uint16 {
	if key == "*" {
		out := make([]uint16, 0, len(tw.locos))
		for addr := range tw.locos {
			out = append(out, addr)
		}
		return out
	}
	addr, _, ok := parseLocoKey(key)
	if !ok {
		return nil
	}
	if _, held := tw.locos[addr]; !held {
		return nil
	}
	return []uint16{addr}
}
