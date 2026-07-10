package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/slotlease"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type diagFakeStation struct{}

func (diagFakeStation) AcquireSlot(commandstation.LocoAddr) error { return nil }
func (diagFakeStation) ReleaseSlot(commandstation.LocoAddr) error { return nil }

func TestSlotsDiagHandler_throttlesBurstEvents(t *testing.T) {
	l := slotlease.New(diagFakeStation{}, nil, nil, nil, nil, slotlease.Config{
		MaxPerUser: 8,
		MaxSlots:   80,
	})
	h := NewSlotsDiagHandler(SlotsDiagConfig{Leaser: l})

	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	readFrame := func() map[string]any {
		t.Helper()
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var frame map[string]any
		if err := json.Unmarshal(data, &frame); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return frame
	}

	if frame := readFrame(); frame["type"] != slotsSnapshotType {
		t.Fatalf("first frame type = %v", frame["type"])
	}

	for i := 0; i < 10; i++ {
		if _, err := l.Select(uint(i+1), "s", "ws", uint16(100+i)); err != nil {
			t.Fatalf("Select %d: %v", i, err)
		}
	}

	extra := 0
	readCtx, readCancel := context.WithTimeout(ctx, 600*time.Millisecond)
	defer readCancel()
	for {
		if _, _, err := conn.Read(readCtx); err != nil {
			break
		}
		extra++
	}
	if extra > 2 {
		t.Fatalf("received %d extra frames during burst, want ≤2", extra)
	}
}

func TestSlotsDiagHandler_ServeRelease(t *testing.T) {
	l := slotlease.New(diagFakeStation{}, nil, nil, nil, nil, slotlease.Config{
		MaxPerUser: 8,
		MaxSlots:   80,
	})
	h := NewSlotsDiagHandler(SlotsDiagConfig{Leaser: l})
	if _, err := l.Select(1, "s", "ws", 77); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(h.ServeRelease))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"addr":77}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if l.LeaseCount() != 0 {
		t.Fatalf("leases = %d, want 0 after ForceRelease", l.LeaseCount())
	}
}
