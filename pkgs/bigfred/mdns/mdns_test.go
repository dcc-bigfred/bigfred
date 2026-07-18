package mdns_test

import (
	"context"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/mdns"
)

func TestRegistrarRegisterLifecycle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	reg := mdns.NewRegistrar(nil)
	err := reg.Register(ctx, mdns.RegisterInput{
		Instance: "TestCS #1",
		Service:  "_withrottle._tcp",
		Port:     41234,
		TXT:      map[string]string{"proto": "withrottle"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
}

func TestRegisterAllMultipleServices(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	reg := mdns.NewRegistrar(nil)
	err := reg.RegisterAll(ctx, []mdns.RegisterInput{
		{Instance: "CS #1", Service: "_withrottle._tcp", Port: 41234},
		{Instance: "CS #1", Service: "_z21._udp", Port: 21105},
	})
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
}

func TestRegisterWithHost(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	reg := mdns.NewRegistrar(nil)
	err := reg.Register(ctx, mdns.RegisterInput{
		Instance: "BigFred",
		Service:  mdns.ServiceHTTP,
		Host:     "bigfred",
		Port:     8080,
	})
	if err != nil {
		t.Fatalf("Register with Host: %v", err)
	}
}

func TestRegisterInvalidPort(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	reg := mdns.NewRegistrar(nil)
	err := reg.Register(ctx, mdns.RegisterInput{
		Instance: "BigFred",
		Service:  mdns.ServiceHTTP,
		Port:     0,
	})
	if err == nil {
		t.Fatal("expected error for port 0")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	t.Parallel()
	cases := []struct {
		host string
		want bool
	}{
		{"", false},
		{"0.0.0.0", false},
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"192.168.1.10", false},
	}
	for _, tc := range cases {
		if got := mdns.IsLoopbackHost(tc.host); got != tc.want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}
