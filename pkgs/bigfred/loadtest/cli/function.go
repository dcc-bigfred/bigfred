package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func parseDCCFunction(raw string) (uint8, error) {
	s := strings.TrimSpace(strings.ToUpper(raw))
	if s == "" {
		return 0, fmt.Errorf("function name is required")
	}
	if !strings.HasPrefix(s, "F") {
		return 0, fmt.Errorf("function must look like F0..F28, got %q", raw)
	}
	n, err := strconv.ParseUint(s[1:], 10, 8)
	if err != nil {
		return 0, fmt.Errorf("function must look like F0..F28, got %q", raw)
	}
	if n > 28 {
		return 0, fmt.Errorf("function must be F0..F28, got %q", raw)
	}
	return uint8(n), nil
}
