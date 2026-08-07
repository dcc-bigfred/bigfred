package service

import (
	"fmt"
	"net"

	miclient "github.com/dcc-bigfred/microinit/go/client"
)

// MicroinitAPI is the subset of the microinit Go client used by admin
// service listing, daemon info, and log streaming.
type MicroinitAPI interface {
	List() ([]miclient.ServiceStatus, error)
	Info() (*miclient.DaemonInfo, error)
	FollowLogs(name string, lines int, follow bool) (net.Conn, error)
	ReadFrame(conn net.Conn) (miclient.Response, error)
}

// MicroinitControl exposes full microinit List/Info/FollowLogs for the
// admin System and Logi pages. It is separate from SystemControl (host
// power) and from ServiceManager.Status() (reduced ServiceState) so the
// ServiceManager interface stays unchanged.
type MicroinitControl struct {
	client MicroinitAPI
}

// NewMicroinitControl wraps a ServiceManager when it is the microinit-backed
// *manager with a live supervisor; otherwise returns a control that always
// reports unavailable (tests / --no-supervisor).
func NewMicroinitControl(mgr ServiceManager) *MicroinitControl {
	m, ok := mgr.(*manager)
	if !ok || m == nil || m.supervisor == nil {
		return &MicroinitControl{}
	}
	return &MicroinitControl{client: m.supervisor.Client()}
}

// NewMicroinitControlWithClient is for tests that inject a fake client.
func NewMicroinitControlWithClient(client MicroinitAPI) *MicroinitControl {
	return &MicroinitControl{client: client}
}

// Available reports whether a microinit client is wired.
func (c *MicroinitControl) Available() bool {
	return c != nil && c.client != nil
}

// ListServices returns the full microinit service catalogue.
func (c *MicroinitControl) ListServices() ([]miclient.ServiceStatus, error) {
	if !c.Available() {
		return nil, ErrSystemUnavailable
	}
	services, err := c.client.List()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSystemUnavailable, err)
	}
	return services, nil
}

// Info returns microinit DaemonInfo.
func (c *MicroinitControl) Info() (*miclient.DaemonInfo, error) {
	if !c.Available() {
		return nil, ErrSystemUnavailable
	}
	info, err := c.client.Info()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSystemUnavailable, err)
	}
	return info, nil
}

// FollowLogs opens a microinit log stream (history and/or follow).
func (c *MicroinitControl) FollowLogs(name string, lines int, follow bool) (net.Conn, error) {
	if !c.Available() {
		return nil, ErrSystemUnavailable
	}
	return c.client.FollowLogs(name, lines, follow)
}

// ReadFrame reads the next non-heartbeat frame from a FollowLogs connection.
func (c *MicroinitControl) ReadFrame(conn net.Conn) (miclient.Response, error) {
	if !c.Available() {
		return miclient.Response{}, ErrSystemUnavailable
	}
	return c.client.ReadFrame(conn)
}
