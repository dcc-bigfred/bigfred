package remotes_test

import (
	"context"
	"testing"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

type recordingObserver struct {
	count int
}

func (o *recordingObserver) OnLocoStateChanged(_ context.Context, _ contract.LocoStateWire, _ string) {
	o.count++
}

func TestLocoStateNotifier(t *testing.T) {
	t.Parallel()
	n := remotes.NewLocoStateNotifier()
	obs := &recordingObserver{}
	n.Register(obs)
	n.Register(obs)

	snap := contract.LocoStateWire{Address: 3, Speed: 10}
	n.Notify(context.Background(), snap, "")
	if obs.count != 1 {
		t.Fatalf("observer count = %d, want 1 (no duplicate registration)", obs.count)
	}
}

func TestLocoStateNotifierAnyRegistered(t *testing.T) {
	t.Parallel()
	n := remotes.NewLocoStateNotifier()
	if n.AnyRegistered() {
		t.Fatal("empty notifier should report no observers")
	}
	n.Register(&recordingObserver{})
	if !n.AnyRegistered() {
		t.Fatal("registered observer should be visible")
	}
}
