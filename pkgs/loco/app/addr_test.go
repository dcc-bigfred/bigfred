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
