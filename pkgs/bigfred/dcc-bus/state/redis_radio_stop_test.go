package state

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

func TestPublishLayoutRadioStop(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	const layoutID uint = 9
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rs := NewRedis(client, layoutID, 1)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	sub := client.Subscribe(ctx, contract.LayoutRadioStopChannel(layoutID))
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	ch := sub.Channel()

	want := contract.RadioStopCommandWire{
		TriggeredByUserID: 42,
		TriggeredByLogin:  "handset",
		At:                1_700_000_000_000,
	}
	if err := rs.PublishLayoutRadioStop(ctx, want); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-ch:
		var got contract.RadioStopCommandWire
		if err := json.Unmarshal([]byte(msg.Payload), &got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("payload = %+v, want %+v", got, want)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for radio stop publish")
	}
}
