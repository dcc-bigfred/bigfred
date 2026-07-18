package discovery

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/z21server"
	"github.com/keskad/loco/pkgs/bigfred/mdns"
)

const (
	ServiceWithrottle = "_withrottle._tcp"
	ServiceZ21        = "_z21._udp"
)

// ServiceConfig describes one handset protocol to advertise.
type ServiceConfig struct {
	Protocol         string
	LayoutID         uint
	CommandStationID uint
	InstanceName     string
	Port             int
	Serial           uint32 // Z21 only; 0 derives from layout+CS ids
}

// Z21BeaconConfig enables the periodic Z21 UDP discovery beacon.
type Z21BeaconConfig struct {
	Port             int
	LayoutID         uint
	CommandStationID uint
	Serial           uint32
}

// RunConfig groups every discovery mechanism for one dcc-bus process.
type RunConfig struct {
	Services  []ServiceConfig
	Z21Beacon *Z21BeaconConfig
}

// RunOption configures optional Run behaviour (mainly for tests).
type RunOption func(*runOptions)

type runOptions struct {
	beaconDest *net.UDPAddr
}

// WithBeaconDest overrides the Z21 beacon destination (tests only).
func WithBeaconDest(addr *net.UDPAddr) RunOption {
	return func(o *runOptions) {
		o.beaconDest = addr
	}
}

// Run starts a single mDNS responder for all services and an optional Z21 beacon.
func Run(ctx context.Context, cfg RunConfig, log logrus.FieldLogger, opts ...RunOption) error {
	if log == nil {
		log = logrus.New()
	}
	var o runOptions
	for _, opt := range opts {
		opt(&o)
	}

	reg := mdns.NewRegistrar(log)
	registrations := make([]mdns.RegisterInput, 0, len(cfg.Services))
	for _, svc := range cfg.Services {
		in, err := registerInputForService(svc)
		if err != nil {
			return err
		}
		registrations = append(registrations, in)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	if len(registrations) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := reg.RegisterAll(ctx, registrations)
			if err != nil && ctx.Err() == nil {
				errCh <- err
			}
		}()
	}

	if cfg.Z21Beacon != nil {
		beacon := cfg.Z21Beacon
		serial := beacon.Serial
		if serial == 0 {
			serial = z21server.VirtualSerial(beacon.LayoutID, beacon.CommandStationID)
		}
		frame := z21server.SerialReply(serial)
		dest := o.beaconDest
		if dest == nil {
			dest = &net.UDPAddr{IP: net.IPv4bcast, Port: beacon.Port}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := runBeacon(ctx, dest, frame, log)
			if err != nil && ctx.Err() == nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		return err
	}
	return nil
}

func registerInputForService(svc ServiceConfig) (mdns.RegisterInput, error) {
	service := ServiceForProtocol(svc.Protocol)
	if service == "" {
		return mdns.RegisterInput{}, fmt.Errorf("discovery: unknown protocol %q", svc.Protocol)
	}
	txt := map[string]string{
		"layoutId":         strconv.FormatUint(uint64(svc.LayoutID), 10),
		"commandStationId": strconv.FormatUint(uint64(svc.CommandStationID), 10),
		"proto":            svc.Protocol,
	}
	if svc.Protocol == contract.RemoteProtocolZ21 {
		serial := svc.Serial
		if serial == 0 {
			serial = z21server.VirtualSerial(svc.LayoutID, svc.CommandStationID)
		}
		txt["serial"] = strconv.FormatUint(uint64(serial), 10)
	}
	instance := svc.InstanceName
	if instance == "" {
		instance = InstanceName("", svc.CommandStationID)
	}
	return mdns.RegisterInput{
		Instance: instance,
		Service:  service,
		Port:     svc.Port,
		TXT:      txt,
	}, nil
}

// ServiceForProtocol returns the DNS-SD service type for a remote protocol.
func ServiceForProtocol(protocol string) string {
	switch protocol {
	case contract.RemoteProtocolWithrottle:
		return ServiceWithrottle
	case contract.RemoteProtocolZ21:
		return ServiceZ21
	default:
		return ""
	}
}
