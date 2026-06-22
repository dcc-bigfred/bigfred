package syntax

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// CVBitAssignment sets or clears one bit in a configuration variable.
type CVBitAssignment struct {
	CVNumber uint16
	Bit      uint8
	Set      bool // true when the bit value is 1
}

var reCVBitArg = regexp.MustCompile(`(?i)^cv(\d+)b(\d+)$`)

// ParseCVBitAssignments parses tokens such as CV29b5=1 or cv1b0=0.
// Tokens may be comma- or whitespace-separated.
func ParseCVBitAssignments(input string) ([]CVBitAssignment, error) {
	tokens := splitCVBitTokens(input)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no CV bit assignments specified (expected CV29b5=1)")
	}

	assignments := make([]CVBitAssignment, 0, len(tokens))
	for _, tok := range tokens {
		a, err := parseCVBitToken(tok)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, nil
}

func splitCVBitTokens(input string) []string {
	fields := strings.FieldsFunc(strings.TrimSpace(input), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func parseCVBitToken(tok string) (CVBitAssignment, error) {
	parts := strings.SplitN(tok, "=", 2)
	if len(parts) != 2 {
		return CVBitAssignment{}, fmt.Errorf("invalid CV bit assignment %q (expected CV29b5=1)", tok)
	}

	m := reCVBitArg.FindStringSubmatch(strings.TrimSpace(parts[0]))
	if m == nil {
		return CVBitAssignment{}, fmt.Errorf("invalid CV bit %q (expected CV<number>b<bit>, e.g. CV29b5)", parts[0])
	}

	cvNum, err := strconv.ParseUint(m[1], 10, 16)
	if err != nil {
		return CVBitAssignment{}, fmt.Errorf("invalid CV number in %q: %w", tok, err)
	}
	bit, err := strconv.ParseUint(m[2], 10, 8)
	if err != nil {
		return CVBitAssignment{}, fmt.Errorf("invalid bit index in %q: %w", tok, err)
	}
	if bit > 7 {
		return CVBitAssignment{}, fmt.Errorf("bit b%d out of range in %q (valid: b0-b7)", bit, tok)
	}

	val, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 8)
	if err != nil {
		return CVBitAssignment{}, fmt.Errorf("invalid bit value in %q: %w", tok, err)
	}
	if val > 1 {
		return CVBitAssignment{}, fmt.Errorf("bit value in %q must be 0 or 1", tok)
	}

	return CVBitAssignment{
		CVNumber: uint16(cvNum),
		Bit:      uint8(bit),
		Set:      val == 1,
	}, nil
}

// ApplyCVBits updates value by applying bit assignments in order.
func ApplyCVBits(value int, assignments []CVBitAssignment) (int, error) {
	for _, a := range assignments {
		if a.Bit > 7 {
			return 0, fmt.Errorf("bit b%d out of range for CV%d (valid: b0-b7)", a.Bit, a.CVNumber)
		}
		mask := 1 << a.Bit
		if a.Set {
			value |= mask
		} else {
			value &^= mask
		}
	}
	return value, nil
}
