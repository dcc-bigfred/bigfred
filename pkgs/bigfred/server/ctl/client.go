package ctl

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// CallError is a daemon {"error":"<code>"} response.
type CallError struct {
	Code string
}

func (e CallError) Error() string { return e.Code }

// Call opens the socket, sends one typed request, reads one response, and
// closes. Success returns the raw JSON payload (REST-shaped).
func Call(socketPath, reqType string) ([]byte, error) {
	conn, err := net.DialTimeout("unix", socketPath, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s: %w (is loco-server running?)", socketPath, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if err := WriteFrame(conn, map[string]string{"type": reqType}); err != nil {
		return nil, err
	}
	raw, err := ReadFrame(conn)
	if err != nil {
		return nil, err
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err == nil && body.Error != "" {
		return nil, CallError{Code: body.Error}
	}
	return raw, nil
}
