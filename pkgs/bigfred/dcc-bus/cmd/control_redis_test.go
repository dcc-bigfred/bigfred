package cmd

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	buserrors "github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
)

func TestLogControlProgramming_publishesRejectionEvent(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rs := state.NewRedis(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 2, 1)
	log := logrus.New()
	log.SetOutput(io.Discard)
	r := &Router{redis: rs, log: log}

	// Subscribe to the daemon's event channel before publishing so the
	// message is not lost to fire-and-forget timing.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub := rs.Client().Subscribe(ctx, contract.DccBusEventChannel(2, 1))
	defer func() { _ = sub.Close() }()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	msgCh := sub.Channel()

	r.logControlProgramming(protocol.TypeLocoCVWrite, Result{
		OK:          false,
		Code:        buserrors.CodeProgrammingDisabled,
		LocoAddress: 47,
	})

	select {
	case msg := <-msgCh:
		var env contract.EnvelopeWire
		if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		if env.Type != protocol.TypeControlProgrammingRejected {
			t.Fatalf("event type = %q, want %q", env.Type, protocol.TypeControlProgrammingRejected)
		}
		var p protocol.ControlProgrammingRejectedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if p.FrameType != protocol.TypeLocoCVWrite {
			t.Errorf("frameType = %q, want %q", p.FrameType, protocol.TypeLocoCVWrite)
		}
		if p.Code != buserrors.CodeProgrammingDisabled {
			t.Errorf("code = %q, want %q", p.Code, buserrors.CodeProgrammingDisabled)
		}
		if p.Address != 47 {
			t.Errorf("address = %d, want 47", p.Address)
		}
	case <-time.After(time.Second):
		t.Fatal("control.programming.rejected event was not published")
	}
}

func TestLogControlProgramming_silentOnSuccess(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rs := state.NewRedis(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 2, 1)
	log := logrus.New()
	log.SetOutput(io.Discard)
	r := &Router{redis: rs, log: log}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub := rs.Client().Subscribe(ctx, contract.DccBusEventChannel(2, 1))
	defer func() { _ = sub.Close() }()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	msgCh := sub.Channel()

	r.logControlProgramming(protocol.TypeLocoCVWrite, OKResult())

	select {
	case msg := <-msgCh:
		t.Fatalf("expected no event on success, got %s", msg.Payload)
	case <-time.After(150 * time.Millisecond):
		// ok — no event published.
	}
}
