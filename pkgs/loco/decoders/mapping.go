package decoders

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// MappingDirection selects which travel direction receives the mapping.
type MappingDirection int

const (
	MappingBoth MappingDirection = iota
	MappingForward
	MappingReverse
)

// MappingOutputKind distinguishes numbered outputs from the dedicated F0 headlight outputs.
type MappingOutputKind int

const (
	OutputNumbered MappingOutputKind = iota
	OutputF0Forward
	OutputF0Reverse
)

// MappingOutput identifies one physical output target in a mapping request.
type MappingOutput struct {
	Kind   MappingOutputKind
	Number uint8  // valid when Kind == OutputNumbered (O/FO/AUX number, >= 1)
	Label  string // canonical label for display (e.g. O2, F0_F)
}

// MappingWrite is one CV assignment for a function-to-output mapping.
type MappingWrite struct {
	CV    uint16
	Value int
}

// OutputMappable programs which physical outputs respond to a function key.
type OutputMappable interface {
	SetFunctionMapping(fn uint8, outputs []MappingOutput, direction MappingDirection) ([]MappingWrite, error)
}

var (
	reFunctionArg = regexp.MustCompile(`(?i)^F(\d+)$`)
	reOutputArg   = regexp.MustCompile(`(?i)^(?:O|FO|AUX)(\d+)$`)
)

// ParseFunctionArg parses a function key argument such as F0 or f12.
func ParseFunctionArg(arg string) (uint8, error) {
	m := reFunctionArg.FindStringSubmatch(strings.TrimSpace(arg))
	if m == nil {
		return 0, fmt.Errorf("invalid function %q (expected F0-F28)", arg)
	}
	fn, err := strconv.ParseUint(m[1], 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid function %q: %w", arg, err)
	}
	if fn > 28 {
		return 0, fmt.Errorf("function F%d out of range (F0-F28)", fn)
	}
	return uint8(fn), nil
}

// ParseOutputArgs parses a comma-separated output list such as O2,AUX4,F0_F.
//
// Supported tokens:
//   - O<n> / FO<n> / AUX<n> — numbered physical output
//   - F0_F / F0F / FL       — F0 forward headlight output
//   - F0_R / F0R / FR       — F0 reverse headlight output
func ParseOutputArgs(arg string) ([]MappingOutput, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, fmt.Errorf("no outputs specified")
	}

	parts := strings.Split(arg, ",")
	outputs := make([]MappingOutput, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out, err := parseOutputToken(part)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[out.Label]; ok {
			continue
		}
		seen[out.Label] = struct{}{}
		outputs = append(outputs, out)
	}

	if len(outputs) == 0 {
		return nil, fmt.Errorf("no outputs specified")
	}
	return outputs, nil
}

func parseOutputToken(part string) (MappingOutput, error) {
	switch strings.ToUpper(part) {
	case "F0_F", "F0F", "FL":
		return MappingOutput{Kind: OutputF0Forward, Label: "F0_F"}, nil
	case "F0_R", "F0R", "FR":
		return MappingOutput{Kind: OutputF0Reverse, Label: "F0_R"}, nil
	}

	m := reOutputArg.FindStringSubmatch(part)
	if m == nil {
		return MappingOutput{}, fmt.Errorf("invalid output %q (expected O2, FO3, AUX4, F0_F or F0_R)", part)
	}
	num, err := strconv.ParseUint(m[1], 10, 8)
	if err != nil {
		return MappingOutput{}, fmt.Errorf("invalid output %q: %w", part, err)
	}
	if num == 0 {
		return MappingOutput{}, fmt.Errorf("output numbers start at 1")
	}
	return MappingOutput{
		Kind:   OutputNumbered,
		Number: uint8(num),
		Label:  fmt.Sprintf("O%d", num),
	}, nil
}

// MappingAssignment is one function key mapped to a set of outputs.
type MappingAssignment struct {
	Function uint8
	Outputs  []MappingOutput
}

// ParseMappingAssignments parses one or more FUNCTION=OUTPUTS tokens such as
// "F0=O1,O2" "F1=O4,O6". A legacy two-argument form ("F0" "O1,O2") is also
// accepted for backwards compatibility. Later assignments for the same function
// override earlier ones.
func ParseMappingAssignments(args []string) ([]MappingAssignment, error) {
	if len(args) == 2 && !strings.Contains(args[0], "=") && !strings.Contains(args[1], "=") {
		fn, err := ParseFunctionArg(args[0])
		if err != nil {
			return nil, err
		}
		outputs, err := ParseOutputArgs(args[1])
		if err != nil {
			return nil, err
		}
		return []MappingAssignment{{Function: fn, Outputs: outputs}}, nil
	}

	tokens := splitWhitespaceArgs(args)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no mapping specified (expected F0=O1,O2 …)")
	}

	assignments := make([]MappingAssignment, 0, len(tokens))
	seen := make(map[uint8]int, len(tokens))
	for _, tok := range tokens {
		parts := strings.SplitN(tok, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping %q (expected FUNCTION=OUTPUTS, e.g. F0=O1,O2)", tok)
		}
		fn, err := ParseFunctionArg(parts[0])
		if err != nil {
			return nil, err
		}
		outputs, err := ParseOutputArgs(parts[1])
		if err != nil {
			return nil, err
		}
		if idx, ok := seen[fn]; ok {
			assignments[idx].Outputs = outputs
			continue
		}
		seen[fn] = len(assignments)
		assignments = append(assignments, MappingAssignment{Function: fn, Outputs: outputs})
	}
	return assignments, nil
}

// splitWhitespaceArgs splits each argument on whitespace and flattens the result,
// so a quoted "F0=O1 F1=O2" behaves like two separate shell arguments.
func splitWhitespaceArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, strings.Fields(a)...)
	}
	return out
}

// splitListArgs splits arguments on both commas and whitespace and flattens the
// result, so "O1=10, O6=50" and "O1=10,O6=50" parse identically.
func splitListArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		fields := strings.FieldsFunc(a, func(r rune) bool {
			return r == ',' || unicode.IsSpace(r)
		})
		out = append(out, fields...)
	}
	return out
}

// DetectMapping returns a decoder-specific OutputMappable implementation.
func DetectMapping(cv CVAccess) (OutputMappable, error) {
	id, err := Identify(cv)
	if err != nil {
		return nil, err
	}

	switch id.Kind {
	case DecoderZIMO:
		return NewZIMOMS450Mapping(cv), nil
	case DecoderRailBOX:
		locomotive, locoErr := railboxIsLocomotive(cv)
		if locoErr == nil && locomotive {
			return nil, fmt.Errorf("RB 23xx output mapping uses map.txt; use RailBOX Railroad Control or Wi-Fi upload")
		}
		return NewRailboxRB2112Mapping(cv), nil
	case DecoderESU:
		return nil, fmt.Errorf("ESU LokSound 5 mapping uses indexed CV pages; use LokProgrammer")
	default:
		return nil, fmt.Errorf("output mapping is not supported for decoder %s (CV7=%d, CV8=%d)", id.Name, id.SoftwareVersion, id.ManufacturerID)
	}
}
