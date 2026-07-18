package discovery_test

import (
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/discovery"
)

func TestServiceForProtocol(t *testing.T) {
	t.Parallel()
	if got := discovery.ServiceForProtocol("withrottle"); got != discovery.ServiceWithrottle {
		t.Fatalf("withrottle: %q", got)
	}
	if got := discovery.ServiceForProtocol("z21"); got != discovery.ServiceZ21 {
		t.Fatalf("z21: %q", got)
	}
	if got := discovery.ServiceForProtocol("unknown"); got != "" {
		t.Fatalf("unknown: %q", got)
	}
}

func TestInstanceName(t *testing.T) {
	t.Parallel()
	if got := discovery.InstanceName("Main", 3); got != "Main #3" {
		t.Fatalf("got %q", got)
	}
	if got := discovery.InstanceName("", 5); got != "BigFred #5" {
		t.Fatalf("got %q", got)
	}
}
