package netutil

import (
	"net"
	"testing"
)

func TestIPv4PrefixFromAddrs(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
		&net.IPNet{IP: net.ParseIP("169.254.1.1"), Mask: net.CIDRMask(16, 32)},
		&net.IPNet{IP: net.ParseIP("192.168.0.15"), Mask: net.CIDRMask(24, 32)},
	}
	got, err := ipv4PrefixFromAddrs(addrs)
	if err != nil {
		t.Fatal(err)
	}
	if got != "192.168.0" {
		t.Fatalf("got %q", got)
	}
}

func TestIPv4PrefixFallbackNon24(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("10.1.2.3"), Mask: net.CIDRMask(16, 32)},
	}
	got, err := ipv4PrefixFromAddrs(addrs)
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.1.2" {
		t.Fatalf("got %q", got)
	}
}

func TestIPv4PrefixNone(t *testing.T) {
	_, err := ipv4PrefixFromAddrs([]net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
