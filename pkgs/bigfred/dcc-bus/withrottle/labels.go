package withrottle

import (
	"fmt"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func buildFunctionLabelLine(throttleID byte, locoKey string, defs []contract.FunctionDefinition) string {
	if len(defs) == 0 {
		return ""
	}
	maxFn := 0
	labels := make([]string, maxWiThrottleFunction+1)
	for _, d := range defs {
		n := int(d.Num)
		if n < 0 || n > maxWiThrottleFunction {
			continue
		}
		if n > maxFn {
			maxFn = n
		}
		labels[n] = d.FunctionLabel()
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("M%cL%s%s", throttleID, locoKey, propSep))
	for fn := 0; fn <= maxFn; fn++ {
		b.WriteString(entrySep)
		b.WriteString(labels[fn])
	}
	return b.String()
}

// buildAcquireReply returns the verbose state dump after a successful acquire.
func buildAcquireReply(throttleID byte, addr uint16, defs []contract.FunctionDefinition) []string {
	id := string(throttleID)
	key := locoKeyForAddr(addr)
	lines := []string{
		fmt.Sprintf("M%s+%s%s", id, key, propSep),
	}
	if labelLine := buildFunctionLabelLine(throttleID, key, defs); labelLine != "" {
		lines = append(lines, labelLine)
	}
	for fn := 0; fn <= maxWiThrottleFunction; fn++ {
		lines = append(lines, fmt.Sprintf("M%sA%s%sF0%d", id, key, propSep, fn))
	}
	lines = append(lines,
		fmt.Sprintf("M%sA%s%sV0", id, key, propSep),
		fmt.Sprintf("M%sA%s%sR1", id, key, propSep),
		fmt.Sprintf("M%sA%s%ss1", id, key, propSep),
	)
	return lines
}
