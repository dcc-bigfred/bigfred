package decoders

import "testing"

func TestParseFunctionArg(t *testing.T) {
	fn, err := ParseFunctionArg("F0")
	if err != nil || fn != 0 {
		t.Fatalf("F0: fn=%d err=%v", fn, err)
	}
	fn, err = ParseFunctionArg("f12")
	if err != nil || fn != 12 {
		t.Fatalf("f12: fn=%d err=%v", fn, err)
	}
	if _, err := ParseFunctionArg("X1"); err == nil {
		t.Fatal("expected error for X1")
	}
}

func TestParseOutputArgs(t *testing.T) {
	outputs, err := ParseOutputArgs("F0_F,O2,O4")
	if err != nil {
		t.Fatalf("ParseOutputArgs: %v", err)
	}
	if len(outputs) != 3 {
		t.Fatalf("got %v", outputs)
	}
	if outputs[0].Kind != OutputF0Forward || outputs[0].Label != "F0_F" {
		t.Fatalf("outputs[0]=%+v", outputs[0])
	}
	if outputs[1].Kind != OutputNumbered || outputs[1].Number != 2 {
		t.Fatalf("outputs[1]=%+v", outputs[1])
	}
	if outputs[2].Kind != OutputNumbered || outputs[2].Number != 4 {
		t.Fatalf("outputs[2]=%+v", outputs[2])
	}

	outputs, err = ParseOutputArgs("AUX1,FO2,aux2,f0_r")
	if err != nil {
		t.Fatalf("aliases: %v", err)
	}
	if len(outputs) != 3 {
		t.Fatalf("got %v", outputs)
	}
	if outputs[0].Number != 1 || outputs[1].Number != 2 {
		t.Fatalf("got %v", outputs)
	}
	if outputs[2].Kind != OutputF0Reverse {
		t.Fatalf("outputs[2]=%+v", outputs[2])
	}
}

func TestParseMappingAssignments(t *testing.T) {
	assignments, err := ParseMappingAssignments([]string{"F0=O1,O2", "F1=O4,O6"})
	if err != nil {
		t.Fatalf("ParseMappingAssignments: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("got %d assignments", len(assignments))
	}
	if assignments[0].Function != 0 || len(assignments[0].Outputs) != 2 {
		t.Fatalf("assignment[0]=%+v", assignments[0])
	}
	if assignments[1].Function != 1 || assignments[1].Outputs[0].Number != 4 {
		t.Fatalf("assignment[1]=%+v", assignments[1])
	}

	// quoted single argument splits on whitespace
	assignments, err = ParseMappingAssignments([]string{"F0=O2 F3=O4"})
	if err != nil {
		t.Fatalf("quoted: %v", err)
	}
	if len(assignments) != 2 || assignments[1].Function != 3 {
		t.Fatalf("quoted got %+v", assignments)
	}

	// legacy two-argument form
	assignments, err = ParseMappingAssignments([]string{"F0", "F0_F"})
	if err != nil {
		t.Fatalf("legacy: %v", err)
	}
	if len(assignments) != 1 || assignments[0].Outputs[0].Kind != OutputF0Forward {
		t.Fatalf("legacy got %+v", assignments)
	}

	if _, err := ParseMappingAssignments([]string{"F0"}); err == nil {
		t.Fatal("expected error for missing outputs")
	}
}

func TestParseBrightnessArgs(t *testing.T) {
	settings, err := ParseBrightnessArgs([]string{"O1=50,O2=5"})
	if err != nil {
		t.Fatalf("ParseBrightnessArgs: %v", err)
	}
	if len(settings) != 2 {
		t.Fatalf("got %d settings", len(settings))
	}
	if settings[0].Output != 1 || settings[0].Percent != 50 {
		t.Fatalf("settings[0]=%+v", settings[0])
	}
	if settings[1].Output != 2 || settings[1].Percent != 5 {
		t.Fatalf("settings[1]=%+v", settings[1])
	}

	// spread across args, with spaces and bare numbers
	settings, err = ParseBrightnessArgs([]string{"O1=10,", "O6=50", "3=20"})
	if err != nil {
		t.Fatalf("spread: %v", err)
	}
	if len(settings) != 3 || settings[2].Output != 3 || settings[2].Percent != 20 {
		t.Fatalf("spread got %+v", settings)
	}

	if _, err := ParseBrightnessArgs([]string{"O1"}); err == nil {
		t.Fatal("expected error for missing percent")
	}
	if _, err := ParseBrightnessArgs([]string{"O1=150"}); err == nil {
		t.Fatal("expected error for percent > 100")
	}
}

func TestZimoOutputBits(t *testing.T) {
	value, err := zimoOutputBits(numberedOutputs(1, 2, 4))
	if err != nil {
		t.Fatalf("zimoOutputBits: %v", err)
	}
	if value != 1|2|8 {
		t.Fatalf("got %d, want %d", value, 1|2|8)
	}

	if _, err := zimoOutputBits([]MappingOutput{{Kind: OutputF0Forward, Label: "F0_F"}}); err == nil {
		t.Fatal("expected error for F0_F on ZIMO")
	}
}

func TestRB2112SplitOutputs(t *testing.T) {
	bank1, bank2, err := rb2112SplitOutputs([]MappingOutput{
		{Kind: OutputF0Forward, Label: "F0_F"},
		{Kind: OutputNumbered, Number: 2, Label: "O2"},
		{Kind: OutputNumbered, Number: 4, Label: "O4"},
	})
	if err != nil {
		t.Fatalf("rb2112SplitOutputs: %v", err)
	}
	if bank2 >= 0 {
		t.Fatalf("unexpected bank2 %d", bank2)
	}
	if bank1 != 1|(1<<2)|(1<<4) {
		t.Fatalf("bank1=%d, want %d", bank1, 1|(1<<2)|(1<<4))
	}

	bank1, _, err = rb2112SplitOutputs([]MappingOutput{{Kind: OutputF0Reverse, Label: "F0_R"}})
	if err != nil {
		t.Fatalf("rb2112SplitOutputs: %v", err)
	}
	if bank1 != 2 {
		t.Fatalf("F0_R bank1=%d, want 2", bank1)
	}

	_, _, err = rb2112SplitOutputs([]MappingOutput{{Kind: OutputNumbered, Number: 1, Label: "O1"}})
	if err == nil {
		t.Fatal("expected error for O1 (must use F0_F/F0_R)")
	}

	_, bank2, err = rb2112SplitOutputs([]MappingOutput{{Kind: OutputNumbered, Number: 8, Label: "O8"}})
	if err != nil {
		t.Fatalf("rb2112SplitOutputs O8: %v", err)
	}
	if bank2 != 1 {
		t.Fatalf("O8 bank2=%d, want 1", bank2)
	}
}

func numberedOutputs(nums ...uint8) []MappingOutput {
	outputs := make([]MappingOutput, len(nums))
	for i, n := range nums {
		outputs[i] = MappingOutput{Kind: OutputNumbered, Number: n, Label: "O" + string(rune('0'+n))}
	}
	return outputs
}

func TestRB2112MappingCVNumbers(t *testing.T) {
	if cv := rb2112MappingCV(0, true, 1); cv != 120 {
		t.Fatalf("F0 forward bank1 CV=%d, want 120", cv)
	}
	if cv := rb2112MappingCV(0, false, 1); cv != 121 {
		t.Fatalf("F0 reverse bank1 CV=%d, want 121", cv)
	}
	if cv := rb2112MappingCV(1, true, 1); cv != 122 {
		t.Fatalf("F1 forward bank1 CV=%d, want 122", cv)
	}
	if cv := rb2112MappingCV(0, true, 2); cv != 190 {
		t.Fatalf("F0 forward bank2 CV=%d, want 190", cv)
	}
}

func TestZIMOMS450MappingSetFunctionMapping(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	m := NewZIMOMS450Mapping(cv)

	writes, err := m.SetFunctionMapping(0, numberedOutputs(1, 2, 4), MappingBoth)
	if err != nil {
		t.Fatalf("SetFunctionMapping: %v", err)
	}
	if len(writes) != 2 {
		t.Fatalf("writes=%d, want 2", len(writes))
	}
	if cv.values[33] != 1|2|8 || cv.values[34] != 1|2|8 {
		t.Fatalf("cv33=%d cv34=%d", cv.values[33], cv.values[34])
	}
}

func TestRailboxRB2112MappingSetFunctionMapping(t *testing.T) {
	cv := &fakeCV{values: map[uint16]int{}}
	m := NewRailboxRB2112Mapping(cv)

	writes, err := m.SetFunctionMapping(0, []MappingOutput{
		{Kind: OutputF0Forward, Label: "F0_F"},
		{Kind: OutputNumbered, Number: 2, Label: "O2"},
		{Kind: OutputNumbered, Number: 4, Label: "O4"},
	}, MappingBoth)
	if err != nil {
		t.Fatalf("SetFunctionMapping: %v", err)
	}
	if len(writes) != 2 {
		t.Fatalf("writes=%d, want 2", len(writes))
	}
	want := 1 | (1 << 2) | (1 << 4)
	if cv.values[120] != want || cv.values[121] != want {
		t.Fatalf("cv120=%d cv121=%d, want %d", cv.values[120], cv.values[121], want)
	}

	cv2 := &fakeCV{values: map[uint16]int{}}
	m2 := NewRailboxRB2112Mapping(cv2)
	if _, err := m2.SetFunctionMapping(0, []MappingOutput{{Kind: OutputF0Forward, Label: "F0_F"}}, MappingForward); err != nil {
		t.Fatalf("F0_F forward: %v", err)
	}
	if cv2.values[120] != 1 {
		t.Fatalf("cv120=%d, want 1", cv2.values[120])
	}
	if _, ok := cv2.values[121]; ok {
		t.Fatalf("cv121 should not be written for forward-only")
	}
}
