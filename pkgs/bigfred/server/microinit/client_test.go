package microinit

import (
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
)

func TestClientListOverUnixSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "microinit.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var req request
		if err := readFrame(conn, &req); err != nil {
			return
		}
		if req.Type != "list" {
			return
		}
		_ = writeFrame(conn, Response{Type: "list", Services: []ServiceStatus{{Name: "redis", State: "running", Enabled: true}}})
	}()
	got, err := (&Client{Socket: socket}).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "redis" || got[0].State != "running" {
		raw, _ := json.Marshal(got)
		t.Fatalf("unexpected list: %s", raw)
	}
}
