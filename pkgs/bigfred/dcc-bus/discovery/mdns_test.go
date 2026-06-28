package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/discovery"
)

func TestRegistrarRegisterLifecycle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	reg := discovery.NewRegistrar(nil)
	err := reg.Register(ctx, discovery.RegisterInput{
		Instance: "TestCS #1",
		Service:  discovery.ServiceWithrottle,
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

	reg := discovery.NewRegistrar(nil)
	err := reg.RegisterAll(ctx, []discovery.RegisterInput{
		{Instance: "CS #1", Service: discovery.ServiceWithrottle, Port: 41234},
		{Instance: "CS #1", Service: discovery.ServiceZ21, Port: 21105},
	})
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
}

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
