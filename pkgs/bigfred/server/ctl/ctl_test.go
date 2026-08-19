package ctl

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/version"
)

type fakeLayouts struct {
	rows []domain.Layout
	err  error
}

func (f fakeLayouts) ListAll(context.Context) ([]domain.Layout, error) {
	return f.rows, f.err
}

type fakeStations struct {
	rows []domain.CommandStation
	err  error
}

func (f fakeStations) ListAll(context.Context) ([]domain.CommandStation, error) {
	return f.rows, f.err
}

type fakeDccBus struct {
	byID map[uint][]service.DccBusProgramStatus
	err  error
}

func (f fakeDccBus) ProgramsForCommandStation(_ context.Context, id uint) ([]service.DccBusProgramStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byID[id], nil
}

func startCtl(t *testing.T, h Handler) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bigfred.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		_ = os.Remove(path)
	})
	go func() { _ = Serve(ctx, ln, h, nil) }()
	return path
}

func TestListenRefusesLivePeer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bigfred.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	_, err = Listen(path)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Listen: %v", err)
	}
}

func TestListenReplacesStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bigfred.sock")
	leftover, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ul, ok := leftover.(*net.UnixListener)
	if !ok {
		t.Fatalf("listener type %T", leftover)
	}
	ul.SetUnlinkOnClose(false)
	if err := leftover.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected leftover inode: %v", err)
	}

	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
}

func TestCallVersion(t *testing.T) {
	path := startCtl(t, Handler{})
	raw, err := Call(path, "version")
	if err != nil {
		t.Fatal(err)
	}
	var info version.Info
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatal(err)
	}
	if info.Product != "bigfred" {
		t.Fatalf("product=%q", info.Product)
	}
}

func TestCallLayoutsList(t *testing.T) {
	path := startCtl(t, Handler{
		Layouts: fakeLayouts{rows: []domain.Layout{{ID: 1, Name: "Klubowa", RadioChatEnabled: true}}},
	})
	raw, err := Call(path, "layouts_list")
	if err != nil {
		t.Fatal(err)
	}
	var rows []protocol.LayoutResponse
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "Klubowa" || rows[0].ID != 1 {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestCallDccBusList(t *testing.T) {
	path := startCtl(t, Handler{
		Stations: fakeStations{rows: []domain.CommandStation{{ID: 5, Name: "CS"}}},
		DccBus: fakeDccBus{byID: map[uint][]service.DccBusProgramStatus{
			5: {{
				LayoutID: 2, LayoutName: "Klubowa", Name: "dcc-bus-2-5",
				Status: "running", Running: true, CommandStationID: 5,
				WithrottleEnabled: true, WithrottlePort: 12091,
			}},
		}},
	})
	raw, err := Call(path, "dcc_bus_list")
	if err != nil {
		t.Fatal(err)
	}
	var body DccBusListResponse
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Programs) != 1 || body.Programs[0].WithrottlePort != 12091 {
		t.Fatalf("programs=%+v", body.Programs)
	}
}

func TestCallDccBusUnavailable(t *testing.T) {
	path := startCtl(t, Handler{})
	_, err := Call(path, "dcc_bus_list")
	var ce CallError
	if !errors.As(err, &ce) || ce.Code != "service_unavailable" {
		t.Fatalf("err=%v", err)
	}
}

func TestCallUnknownType(t *testing.T) {
	path := startCtl(t, Handler{})
	_, err := Call(path, "nope")
	var ce CallError
	if !errors.As(err, &ce) || ce.Code != "invalid_request" {
		t.Fatalf("err=%v", err)
	}
}

func TestCallMissingDaemon(t *testing.T) {
	_, err := Call(filepath.Join(t.TempDir(), "missing.sock"), "version")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListenMode0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bigfred.sock")
	ln, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", st.Mode().Perm())
	}
}

func TestFrameRoundtrip(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	go func() { _ = WriteFrame(a, map[string]string{"type": "version"}) }()
	raw, err := ReadFrame(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"type":"version"}` {
		t.Fatalf("raw=%s", raw)
	}
}

func TestStartCtlAcceptsQuickly(t *testing.T) {
	// Smoke: Call after Serve starts should not need a long sleep.
	path := startCtl(t, Handler{})
	deadline := time.Now().Add(2 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		_, err = Call(path, "version")
		if err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(err)
}
