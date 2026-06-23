package wsutil

import "testing"

func TestDccBusProxyWS(t *testing.T) {
	got, err := DccBusProxyWS("http://192.168.0.86:8080", 2)
	if err != nil {
		t.Fatal(err)
	}
	want := "ws://192.168.0.86:8080/api/v1/dcc-bus/2/ws"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestControlWS(t *testing.T) {
	got, err := ControlWS("https://bigfred.example")
	if err != nil {
		t.Fatal(err)
	}
	want := "wss://bigfred.example/api/v1/ws"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
