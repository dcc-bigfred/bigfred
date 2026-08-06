package platform

import "testing"

func TestSupportsLocoNetSerialMatchesAndroid(t *testing.T) {
	got := SupportsLocoNetSerial()
	want := !IsAndroid()
	if got != want {
		t.Fatalf("SupportsLocoNetSerial() = %v, want %v (IsAndroid=%v)", got, want, IsAndroid())
	}
}

func TestSupportsMicrodnsMatchesAndroid(t *testing.T) {
	got := SupportsMicrodns()
	want := !IsAndroid()
	if got != want {
		t.Fatalf("SupportsMicrodns() = %v, want %v (IsAndroid=%v)", got, want, IsAndroid())
	}
}
