package z21server

import (
	"net"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes/inbound"
)

func TestRegistryTouchIPStickinessReusesSessionOnPortChange(t *testing.T) {
	t.Parallel()
	reg := NewRegistry(nil, nil)
	now := time.Now().UTC()
	addr1 := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 214), Port: 60495}
	addr2 := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 214), Port: 60512}

	c1 := reg.Touch(addr1, now, true)
	c2 := reg.Touch(addr2, now.Add(time.Second), true)

	if c1.Key != inbound.ClientKey(contract.RemoteProtocolZ21, "192.168.0.214") || c2.Key != c1.Key {
		t.Fatalf("keys = %q / %q, want sticky IP key", c1.Key, c2.Key)
	}
	if c2.Addr.Port != 60512 {
		t.Fatalf("Addr port = %d, want updated reply target 60512", c2.Addr.Port)
	}
	if reg.Len() != 1 {
		t.Fatalf("registry len = %d, want 1", reg.Len())
	}
}

func TestRegistryTouchWithoutStickinessUsesPort(t *testing.T) {
	t.Parallel()
	reg := NewRegistry(nil, nil)
	now := time.Now().UTC()
	addr1 := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 214), Port: 60495}
	addr2 := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 214), Port: 60512}

	c1 := reg.Touch(addr1, now, false)
	c2 := reg.Touch(addr2, now, false)

	if c1.Key == c2.Key {
		t.Fatalf("expected distinct keys, both %q", c1.Key)
	}
	if reg.Len() != 2 {
		t.Fatalf("registry len = %d, want 2", reg.Len())
	}
}
