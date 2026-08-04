package microinit

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/datadir"
)

const (
	// DefaultSocket is the hub path when the data root is /data.
	DefaultSocket  = "/data/run/microinit.sock"
	maxFrameBytes  = 16 * 1024 * 1024
	defaultTimeout = 10 * time.Second
)

// DefaultSocketPath returns `$BIGFRED_DATA_DIR/run/microinit.sock` (or DATA_DIR /data).
func DefaultSocketPath() string {
	return datadir.Path("run", "microinit.sock")
}

var (
	ErrInvalidName   = errors.New("invalid service name")
	ErrInvalidAction = errors.New("invalid action")
	ErrNotFound      = errors.New("service not found")
)

type ServiceStatus struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	PID      *int32 `json:"pid"`
	Restarts uint32 `json:"restarts"`
	Enabled  bool   `json:"enabled"`
}

type LogLine struct {
	TS      string `json:"ts"`
	Service string `json:"service"`
	Level   string `json:"level"`
	Msg     string `json:"msg"`
}

type request struct {
	Type   string  `json:"type"`
	Name   string  `json:"name,omitempty"`
	Follow *bool   `json:"follow,omitempty"`
	Lines  *uint64 `json:"lines,omitempty"`
	Mode   string  `json:"mode,omitempty"`
}

type Response struct {
	Type     string          `json:"type"`
	Message  string          `json:"message,omitempty"`
	Services []ServiceStatus `json:"services,omitempty"`
	Status   *ServiceStatus  `json:"status,omitempty"`
	Line     *LogLine        `json:"line,omitempty"`
}

type Client struct {
	Socket  string
	Timeout time.Duration
	Dial    func(network, address string, timeout time.Duration) (net.Conn, error)
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return defaultTimeout
}
func (c *Client) socketPath() string {
	if c.Socket != "" {
		return c.Socket
	}
	return DefaultSocketPath()
}
func (c *Client) dial() (net.Conn, error) {
	dial := c.Dial
	if dial == nil {
		dial = net.DialTimeout
	}
	conn, err := dial("unix", c.socketPath(), c.timeout())
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", c.socketPath(), err)
	}
	_ = conn.SetDeadline(time.Now().Add(c.timeout()))
	return conn, nil
}

func (c *Client) List() ([]ServiceStatus, error) {
	var response Response
	if err := c.roundTrip(request{Type: "list"}, &response); err != nil {
		return nil, err
	}
	if response.Type == "error" {
		return nil, responseError(response.Message)
	}
	if response.Type != "list" {
		return nil, fmt.Errorf("unexpected response type %q", response.Type)
	}
	if response.Services == nil {
		return []ServiceStatus{}, nil
	}
	return response.Services, nil
}
func (c *Client) Status(name string) (*ServiceStatus, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	var response Response
	if err := c.roundTrip(request{Type: "status", Name: name}, &response); err != nil {
		return nil, err
	}
	if response.Type == "error" {
		return nil, responseError(response.Message)
	}
	if response.Type != "status" || response.Status == nil {
		return nil, fmt.Errorf("unexpected response type %q", response.Type)
	}
	return response.Status, nil
}
func (c *Client) Control(name, action string) error {
	if err := validateName(name); err != nil {
		return err
	}
	switch action {
	case "start", "stop", "restart":
	default:
		return ErrInvalidAction
	}
	var response Response
	if err := c.roundTrip(request{Type: action, Name: name}, &response); err != nil {
		return err
	}
	if response.Type == "ok" {
		return nil
	}
	if response.Type == "error" {
		return responseError(response.Message)
	}
	return fmt.Errorf("unexpected response type %q", response.Type)
}

// Shutdown requests a halt-mode shutdown. It is used only for a supervise
// instance started by this process.
func (c *Client) Shutdown() error {
	var response Response
	if err := c.roundTrip(request{Type: "shutdown", Mode: "halt"}, &response); err != nil {
		return err
	}
	if response.Type == "ok" {
		return nil
	}
	if response.Type == "error" {
		return responseError(response.Message)
	}
	return fmt.Errorf("unexpected response type %q", response.Type)
}
func (c *Client) FollowLogs(name string, lines int, follow bool) (net.Conn, error) {
	if name != "" {
		if err := validateName(name); err != nil {
			return nil, err
		}
	}
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	f := follow
	req := request{Type: "logs", Name: name, Follow: &f}
	if lines >= 0 {
		n := uint64(lines)
		req.Lines = &n
	}
	if err := writeFrame(conn, req); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}
func ReadResponse(r io.Reader) (Response, error) {
	var response Response
	return response, readFrame(r, &response)
}
func (c *Client) roundTrip(req request, response *Response) error {
	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := writeFrame(conn, req); err != nil {
		return err
	}
	return readFrame(conn, response)
}
func writeFrame(w io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(payload) > maxFrameBytes {
		return errors.New("frame too large")
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}
func readFrame(r io.Reader, value any) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	n := binary.LittleEndian.Uint32(header[:])
	if n > maxFrameBytes {
		return fmt.Errorf("frame length %d too large", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return err
	}
	return json.Unmarshal(payload, value)
}
func responseError(message string) error {
	if strings.Contains(strings.ToLower(message), "unknown") || strings.Contains(strings.ToLower(message), "not found") {
		return ErrNotFound
	}
	if message == "" {
		message = "microinit request failed"
	}
	return errors.New(message)
}
func validateName(name string) error {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return ErrInvalidName
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return ErrInvalidName
	}
	return nil
}
