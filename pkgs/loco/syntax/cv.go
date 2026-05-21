package syntax

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type CVEntry struct {
	Number uint16
	Value  uint16
}

// ParseCVString parses input string to array of CVEntry (CV number and value)
func ParseCVString(input string, separator string) ([]CVEntry, error) {
	if separator == "" {
		separator = "\n"
	}

	var result []CVEntry
	unique := make(map[uint16]uint16)
	lines := strings.Split(input, separator)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// remove inline comment
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		var cvNum, cvVal string
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			cvNum = strings.TrimSpace(parts[0])
			cvVal = strings.TrimSpace(parts[1])
		} else {
			cvNum = strings.TrimSpace(line)
			cvVal = "0" // default value when no value is provided
		}

		// Support ranges cvX-cvY
		cvNumLower := strings.ToLower(cvNum)
		if strings.Contains(cvNumLower, "-") {
			rangeParts := strings.SplitN(cvNumLower, "-", 2)
			startStr := strings.TrimPrefix(strings.TrimSpace(rangeParts[0]), "cv")
			endStr := strings.TrimPrefix(strings.TrimSpace(rangeParts[1]), "cv")
			startNum, err1 := strconv.ParseUint(startStr, 10, 16)
			endNum, err2 := strconv.ParseUint(endStr, 10, 16)
			if err1 != nil || err2 != nil || startNum > endNum {
				return nil, fmt.Errorf("invalid CV range: %s", cvNum)
			}
			val, err := strconv.ParseUint(cvVal, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid CV value: %s", cvVal)
			}
			for i := uint16(startNum); i <= uint16(endNum); i++ {
				unique[i] = uint16(val)
			}
			continue
		}

		// Remove "CV" or "cv" prefix and parse number
		cvNumLower = strings.TrimPrefix(cvNumLower, "cv")
		num, err := strconv.ParseUint(cvNumLower, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid CV number: %s", cvNum)
		}

		// Parse value
		val, err := strconv.ParseUint(cvVal, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid CV value: %s", cvVal)
		}

		unique[uint16(num)] = uint16(val)
	}

	for k, v := range unique {
		result = append(result, CVEntry{Number: k, Value: v})
	}
	// Sort result by CVEntry.Number
	sort.Slice(result, func(i, j int) bool {
		return result[i].Number < result[j].Number
	})
	return result, nil
}
