package app

import "testing"

func TestAddressFromCVs_ShortAddress(t *testing.T) {
	info, err := addressFromCVs(125, 192, 178, 0)
	if err != nil {
		t.Fatalf("addressFromCVs: %v", err)
	}
	if info.Address != 125 || info.Type != "short" {
		t.Fatalf("got address=%d type=%s, want 125 short", info.Address, info.Type)
	}
}

func TestAddressFromCVs_LongAddress(t *testing.T) {
	info, err := addressFromCVs(3, 192, 178, 32)
	if err != nil {
		t.Fatalf("addressFromCVs: %v", err)
	}
	if info.Address != 178 || info.Type != "long" {
		t.Fatalf("got address=%d type=%s, want 178 long", info.Address, info.Type)
	}
}

func TestAddressFromCVs_LongAddressUpperBoundary(t *testing.T) {
	info, err := addressFromCVs(3, 231, 255, 32)
	if err != nil {
		t.Fatalf("addressFromCVs: %v", err)
	}
	if info.Address != 10239 || info.Type != "long" {
		t.Fatalf("got address=%d type=%s, want 10239 long", info.Address, info.Type)
	}
}

func TestAddressFromCVs_LongAddressZero(t *testing.T) {
	info, err := addressFromCVs(3, 192, 0, 32)
	if err != nil {
		t.Fatalf("addressFromCVs: %v", err)
	}
	if info.Address != 0 || info.Type != "long" {
		t.Fatalf("got address=%d type=%s, want 0 long", info.Address, info.Type)
	}
}

func TestAddressCV29For(t *testing.T) {
	// CV29=46: bits 1,2,3,5 set (28 steps, analog, user speed table, long addr)
	const cv29 = 46

	if got := addressCV29For(cv29, false); got != 14 {
		t.Fatalf("short: got cv29=%d, want 14", got)
	}
	if got := addressCV29For(cv29, true); got != 46 {
		t.Fatalf("long unchanged: got cv29=%d, want 46", got)
	}
	if got := addressCV29For(14, true); got != 46 {
		t.Fatalf("enable long: got cv29=%d, want 46", got)
	}
}

func TestAddressToCVString_ShortAddress(t *testing.T) {
	cvString, err := AddressToCVString(125, 0)
	if err != nil {
		t.Fatalf("AddressToCVString: %v", err)
	}
	if cvString != "cv1=125, cv17=0, cv18=0, cv29=0" {
		t.Fatalf("got %q", cvString)
	}
}

func TestAddressToCVString_ShortAddressPreservesCV29(t *testing.T) {
	cvString, err := AddressToCVString(125, 46)
	if err != nil {
		t.Fatalf("AddressToCVString: %v", err)
	}
	if cvString != "cv1=125, cv17=0, cv18=0, cv29=14" {
		t.Fatalf("got %q", cvString)
	}
}

func TestAddressToCVString_LongAddress(t *testing.T) {
	cvString, err := AddressToCVString(178, 0)
	if err != nil {
		t.Fatalf("AddressToCVString: %v", err)
	}
	if cvString != "cv17=192, cv18=178, cv29=32" {
		t.Fatalf("got %q", cvString)
	}
}

func TestAddressToCVString_LongAddressPreservesCV29(t *testing.T) {
	cvString, err := AddressToCVString(178, 14)
	if err != nil {
		t.Fatalf("AddressToCVString: %v", err)
	}
	if cvString != "cv17=192, cv18=178, cv29=46" {
		t.Fatalf("got %q", cvString)
	}
}

func TestAddressToCVString_LongAddressUpperBoundary(t *testing.T) {
	cvString, err := AddressToCVString(10239, 0)
	if err != nil {
		t.Fatalf("AddressToCVString: %v", err)
	}
	if cvString != "cv17=231, cv18=255, cv29=32" {
		t.Fatalf("got %q", cvString)
	}
}

func TestAddressToCVString_LongAddressZero(t *testing.T) {
	cvString, err := AddressToCVString(0, 0)
	if err != nil {
		t.Fatalf("AddressToCVString: %v", err)
	}
	if cvString != "cv17=192, cv18=0, cv29=32" {
		t.Fatalf("got %q", cvString)
	}
}
