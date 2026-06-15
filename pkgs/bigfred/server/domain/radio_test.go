package domain

import "testing"

func TestValidateTarget(t *testing.T) {
	if err := ValidateTarget(1, 0); err != nil {
		t.Fatalf("user target: %v", err)
	}
	if err := ValidateTarget(0, 2); err != nil {
		t.Fatalf("interlocking target: %v", err)
	}
	if err := ValidateTarget(0, 0); err != ErrRadioInvalidTarget {
		t.Fatalf("none: got %v", err)
	}
	if err := ValidateTarget(1, 2); err != ErrRadioInvalidTarget {
		t.Fatalf("both: got %v", err)
	}
}

func TestValidateContext(t *testing.T) {
	if err := ValidateContext(5, 0); err != nil {
		t.Fatalf("vehicle: %v", err)
	}
	if err := ValidateContext(0, 3); err != nil {
		t.Fatalf("train: %v", err)
	}
	if err := ValidateContext(0, 0); err != ErrRadioInvalidContext {
		t.Fatalf("none: got %v", err)
	}
	if err := ValidateContext(1, 1); err != ErrRadioInvalidContext {
		t.Fatalf("both: got %v", err)
	}
}

func TestValidateNote(t *testing.T) {
	got, err := ValidateNote("  hello  ")
	if err != nil || got != "hello" {
		t.Fatalf("trim: %q %v", got, err)
	}
	long := make([]byte, MaxRadioNoteLen+1)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := ValidateNote(string(long)); err != ErrRadioNoteTooLong {
		t.Fatalf("long note: %v", err)
	}
}

func TestIsValidRadioPhrase(t *testing.T) {
	if !IsValidRadioPhrase(RadioAck) {
		t.Fatal("ACK should be valid")
	}
	if IsValidRadioPhrase(RadioPhrase("NOPE")) {
		t.Fatal("unknown phrase should be invalid")
	}
}
