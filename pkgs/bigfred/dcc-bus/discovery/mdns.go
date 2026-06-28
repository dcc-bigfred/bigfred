// Package discovery advertises inbound handset servers on the LAN.
package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/brutella/dnssd"
	"github.com/sirupsen/logrus"
)

const defaultDomain = "local"

// Registrar announces DNS-SD services until ctx is cancelled.
type Registrar struct {
	log logrus.FieldLogger
}

// NewRegistrar returns a Registrar that logs to log (or discards when nil).
func NewRegistrar(log logrus.FieldLogger) *Registrar {
	if log == nil {
		log = logrus.New()
	}
	return &Registrar{log: log}
}

// RegisterInput describes one DNS-SD service instance.
type RegisterInput struct {
	Instance string
	Service  string // e.g. "_withrottle._tcp" or "_z21._udp"
	Port     int
	TXT      map[string]string
}

// Register blocks, announcing one service until ctx is cancelled.
func (r *Registrar) Register(ctx context.Context, in RegisterInput) error {
	return r.RegisterAll(ctx, []RegisterInput{in})
}

// RegisterAll blocks, announcing every service on one mDNS responder until ctx is cancelled.
func (r *Registrar) RegisterAll(ctx context.Context, services []RegisterInput) error {
	if len(services) == 0 {
		return nil
	}

	ifaces, err := advertiseInterfaces(r.log)
	if err != nil {
		return err
	}

	prepared := make([]dnssd.Config, 0, len(services))
	for _, in := range services {
		cfg, err := validateRegisterInput(in)
		if err != nil {
			return err
		}
		cfg.Domain = defaultDomain
		cfg.Ifaces = ifaces
		prepared = append(prepared, cfg)
	}

	resp, err := dnssd.NewResponder()
	if err != nil {
		return fmt.Errorf("discovery: new responder: %w", err)
	}

	var handles []dnssd.ServiceHandle
	defer func() {
		for _, h := range handles {
			resp.Remove(h)
		}
	}()

	for _, cfg := range prepared {
		srv, err := dnssd.NewService(cfg)
		if err != nil {
			return fmt.Errorf("discovery: new service: %w", err)
		}
		handle, err := resp.Add(srv)
		if err != nil {
			return fmt.Errorf("discovery: add service: %w", err)
		}
		handles = append(handles, handle)
		r.log.WithFields(logrus.Fields{
			"instance": cfg.Name,
			"service":  cfg.Type,
			"port":     cfg.Port,
			"ifaces":   ifaces,
		}).Info("mDNS service registered")
	}

	if err := resp.Respond(ctx); err != nil && ctx.Err() == nil {
		return fmt.Errorf("discovery: respond: %w", err)
	}
	return nil
}

func validateRegisterInput(in RegisterInput) (dnssd.Config, error) {
	if in.Port <= 0 || in.Port > 65535 {
		return dnssd.Config{}, fmt.Errorf("discovery: invalid port %d", in.Port)
	}
	instance := strings.TrimSpace(in.Instance)
	if instance == "" {
		instance = "BigFred"
	}
	serviceType := strings.TrimSpace(in.Service)
	if serviceType == "" {
		return dnssd.Config{}, fmt.Errorf("discovery: service type is required")
	}
	return dnssd.Config{
		Name: instance,
		Type: serviceType,
		Port: in.Port,
		Text: in.TXT,
	}, nil
}

func advertiseInterfaces(log logrus.FieldLogger) ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("discovery: list interfaces: %w", err)
	}
	var names []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 {
			continue
		}
		if skipAdvertiseInterface(iface.Name) {
			continue
		}
		names = append(names, iface.Name)
	}
	if len(names) == 0 {
		if log != nil {
			log.Warn("discovery: no suitable LAN interfaces; advertising on all interfaces")
		}
		return nil, nil
	}
	return names, nil
}

func skipAdvertiseInterface(name string) bool {
	switch {
	case strings.HasPrefix(name, "docker"),
		strings.HasPrefix(name, "veth"),
		strings.HasPrefix(name, "br-"):
		return true
	default:
		return false
	}
}

// InstanceName returns a DNS-SD instance label that stays unique per command station.
func InstanceName(csName string, commandStationID uint) string {
	name := strings.TrimSpace(csName)
	if name == "" {
		name = "BigFred"
	}
	return fmt.Sprintf("%s #%d", name, commandStationID)
}
