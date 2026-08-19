// Package ctl is the Unix-socket control channel for a running loco-server.
//
// Framing matches microinit: 4-byte little-endian length + JSON payload.
// Request bodies are { "type": "…" }. Success payloads are the same JSON as
// the corresponding REST handlers. Errors are {"error":"<code>"}.
package ctl

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const (
	// MaxFrameBytes is the maximum JSON payload (same order as microinit).
	MaxFrameBytes = 16 * 1024 * 1024
	// MaxClients caps concurrent handlers.
	MaxClients = 32
)

// WriteFrame writes one length-prefixed JSON message.
func WriteFrame(w io.Writer, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if len(payload) > MaxFrameBytes {
		return fmt.Errorf("ctl: frame length %d exceeds max %d", len(payload), MaxFrameBytes)
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

// ReadFrame reads one length-prefixed JSON payload (raw bytes).
func ReadFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n > MaxFrameBytes {
		return nil, fmt.Errorf("ctl: frame length %d too large", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeJSON(w io.Writer, v any) error {
	return WriteFrame(w, v)
}

func writeError(w io.Writer, code string) error {
	return WriteFrame(w, map[string]string{"error": code})
}
