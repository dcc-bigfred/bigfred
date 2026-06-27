package inbound

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func mustAddr(t *testing.T, ip string, port int) *net.UDPAddr {
	t.Helper()
	a, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestSubscriberIndexAddRemove(t *testing.T) {
	r := NewClientRegistry()
	c := r.Touch(contract.RemoteProtocolZ21, mustAddr(t, "10.0.0.1", 40000), time.Now().UTC(), false)

	r.SubscribeLoco(c.Key, 3)
	r.SubscribeLoco(c.Key, 7)
	if got := r.Subscribers(3); len(got) != 1 || got[0] != c.Key {
		t.Fatalf("subscribers(3)=%v", got)
	}
	if got := r.Subscribers(7); len(got) != 1 {
		t.Fatalf("subscribers(7)=%v", got)
	}

	r.Remove(c.Key)
	if got := r.Subscribers(3); len(got) != 0 {
		t.Fatalf("subscribers(3) after remove=%v", got)
	}
	if got := r.Subscribers(7); len(got) != 0 {
		t.Fatalf("subscribers(7) after remove=%v", got)
	}
}

// TestSubscriberIndexFIFOEvict verifies that exceeding the 16-subscription
// cap drops the oldest entry from the index (no stale subscriber leaks).
func TestSubscriberIndexFIFOEvict(t *testing.T) {
	r := NewClientRegistry()
	c := r.Touch(contract.RemoteProtocolZ21, mustAddr(t, "10.0.0.2", 40000), time.Now().UTC(), false)

	for addr := uint16(1); addr <= 17; addr++ {
		r.SubscribeLoco(c.Key, addr)
	}
	if got := r.Subscribers(1); len(got) != 0 {
		t.Fatalf("addr 1 should have been evicted from index, got %v", got)
	}
	if got := r.Subscribers(17); len(got) != 1 || got[0] != c.Key {
		t.Fatalf("addr 17 should be subscribed, got %v", got)
	}
}
