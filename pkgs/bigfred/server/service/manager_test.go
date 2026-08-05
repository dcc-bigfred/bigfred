package service_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/microinit"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// TestReplaceServicesSkipsForeign verifies ReplaceServices skips services
// not labeled created-by=bigfred (and now logs the skip), writing only the
// owned services to the drop-in group (P1.3.1 / 3.1).
func TestReplaceServicesSkipsForeign(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "microinit.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleStatus(conn)
		}
	}()

	dropinDir := filepath.Join(t.TempDir(), "dropins")
	mgr, err := service.NewMicroinitManager(service.MicroinitConfig{
		Socket:     sock,
		Bin:        "microinit", // unused: we never spawn (no EnsureRunning call)
		ConfigPath: filepath.Join(t.TempDir(), "microinit.json"),
		DropinDir:  dropinDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	ours := microinit.ServiceDef{Name: "ours", StartCmd: "exec true"}
	foreign := microinit.ServiceDef{Name: "foreign", StartCmd: "exec true"}

	if err := mgr.ReplaceServices(context.Background(), service.GroupDccBus,
		[]microinit.ServiceDef{foreign, ours}); err != nil {
		t.Fatal(err)
	}

	// "ours" should be present; "foreign" must have been skipped.
	if !microinit.DropinExists(dropinDir, service.GroupDccBus, "ours") {
		t.Error("expected owned service 'ours' to be written")
	}
	if microinit.DropinExists(dropinDir, service.GroupDccBus, "foreign") {
		t.Error("expected foreign service 'foreign' to be skipped (not written)")
	}
}

// handleStatus answers status requests: "foreign" is labeled created-by=
// system-image (not ours); "ours" is not found, so CanManage treats it as
// manageable.
func handleStatus(conn net.Conn) {
	defer conn.Close()
	req, err := readFrameMap(conn)
	if err != nil {
		return
	}
	name, _ := req["name"].(string)
	switch name {
	case "foreign":
		_ = writeFrameMap(conn, map[string]any{
			"type": "status",
			"status": map[string]any{
				"name":   "foreign",
				"state":  "running",
				"labels": map[string]any{"created-by": "system-image"},
			},
		})
	case "ours":
		_ = writeFrameMap(conn, map[string]any{
			"type":    "error",
			"message": "unknown service 'ours'",
			"code":    "not_found",
		})
	default:
		_ = writeFrameMap(conn, map[string]any{"type": "error", "message": "unknown", "code": "not_found"})
	}
}

func readFrameMap(r io.Reader) (map[string]any, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(buf, &m)
}

func writeFrameMap(w io.Writer, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

// TestGroupDccBusValue guards the drop-in group name (P2.4.3): it must be the
// stable "dcc-bus" string, not the old "bigfred".
func TestGroupDccBusValue(t *testing.T) {
	if service.GroupDccBus != "dcc-bus" {
		t.Fatalf("GroupDccBus = %q, want %q", service.GroupDccBus, "dcc-bus")
	}
	if service.DccBusGroupName != service.GroupDccBus {
		t.Fatalf("DccBusGroupName = %q, want GroupDccBus (%q)", service.DccBusGroupName, service.GroupDccBus)
	}
}

// Ensure the time import is used (kept for future deadline-based tests).
var _ = time.Second
