package syntax

import "testing"

func TestParseCVBitAssignments(t *testing.T) {
	assignments, err := ParseCVBitAssignments("CV29b5=1, CV29b2=0")
	if err != nil {
		t.Fatalf("ParseCVBitAssignments: %v", err)
	}
	if len(assignments) != 2 {
		t.Fatalf("got %d assignments", len(assignments))
	}
	if assignments[0].CVNumber != 29 || assignments[0].Bit != 5 || !assignments[0].Set {
		t.Fatalf("assignments[0]=%+v", assignments[0])
	}
	if assignments[1].Bit != 2 || assignments[1].Set {
		t.Fatalf("assignments[1]=%+v", assignments[1])
	}
}

func TestParseCVBitAssignmentsRejectsInvalid(t *testing.T) {
	if _, err := ParseCVBitAssignments("CV29=1"); err == nil {
		t.Fatal("expected error for CV29=1")
	}
	if _, err := ParseCVBitAssignments("CV29b8=1"); err == nil {
		t.Fatal("expected error for bit 8")
	}
	if _, err := ParseCVBitAssignments("CV29b5=2"); err == nil {
		t.Fatal("expected error for value 2")
	}
}

func TestApplyCVBits(t *testing.T) {
	got, err := ApplyCVBits(14, []CVBitAssignment{{CVNumber: 29, Bit: 5, Set: true}})
	if err != nil {
		t.Fatalf("ApplyCVBits: %v", err)
	}
	if got != 46 {
		t.Fatalf("got %d, want 46", got)
	}

	got, err = ApplyCVBits(46, []CVBitAssignment{{CVNumber: 29, Bit: 5, Set: false}})
	if err != nil {
		t.Fatalf("ApplyCVBits: %v", err)
	}
	if got != 14 {
		t.Fatalf("got %d, want 14", got)
	}
}
