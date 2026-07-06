package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/auth"
)

func TestSessionSubscribeFIFOOrder(t *testing.T) {
	s := &Session{subscribed: make(map[uint16]struct{}, 4)}
	s.Subscribe(3, 7, 42)
	if oldest, ok := s.OldestSubscribed(); !ok || oldest != 3 {
		t.Fatalf("oldest = %d ok=%v, want 3", oldest, ok)
	}
	s.Unsubscribe(3)
	if oldest, ok := s.OldestSubscribed(); !ok || oldest != 7 {
		t.Fatalf("oldest after unsubscribe = %d ok=%v, want 7", oldest, ok)
	}
}

func TestSessionSendNonBlockingDropOldest(t *testing.T) {
	hold := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusGoingAway, "test")
		<-hold
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "test") })

	s := NewSession(auth.Identity{UserID: 1}, conn)
	t.Cleanup(func() { s.Close("test") })

	for i := 0; i < wsSendQueueCap+50; i++ {
		if err := s.Send(context.Background(), contract.EnvelopeWire{Type: "loco.state"}); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if s.SendDrop() == 0 {
		t.Fatal("expected drops on saturated send queue")
	}
	close(hold)
}

func TestSessionCloseTerminatesWriteLoop(t *testing.T) {
	hold := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusGoingAway, "test")
		<-hold
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	s := NewSession(auth.Identity{UserID: 1}, conn)
	s.Close("done")
	close(hold)

	time.Sleep(20 * time.Millisecond)
	err = s.Send(context.Background(), contract.EnvelopeWire{})
	if err == nil {
		t.Fatal("Send after Close should fail")
	}
}

func TestSessionSendCloseNoRace(t *testing.T) {
	hold := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusGoingAway, "test")
		<-hold
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	s := NewSession(auth.Identity{UserID: 1}, conn)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Send(context.Background(), contract.EnvelopeWire{Type: "loco.state"})
		}()
	}
	s.Close("done")
	close(hold)
	wg.Wait()
}
