// Package netutil provides small Linux-oriented network helpers used by
// one-shot dcc-bus tooling (e.g. `dcc-bus scan`).
package netutil

import (
	"fmt"
	"net"
)

// listAddrs is overridable in tests.
var listAddrs = net.InterfaceAddrs

// LocalIPv4Prefix returns the first three octets of a non-loopback IPv4
// address on this host, e.g. "192.168.0" for 192.168.0.1/24.
// It prefers addresses with a /24-or-larger host mask but accepts any
// IPv4 unicast that is not loopback or link-local.
func LocalIPv4Prefix() (string, error) {
	addrs, err := listAddrs()
	if err != nil {
		return "", fmt.Errorf("netutil: list addresses: %w", err)
	}
	return ipv4PrefixFromAddrs(addrs)
}

func ipv4PrefixFromAddrs(addrs []net.Addr) (string, error) {
	var fallback string
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP == nil {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		prefix := fmt.Sprintf("%d.%d.%d", ip[0], ip[1], ip[2])
		ones, bits := ipNet.Mask.Size()
		if bits == 32 && ones == 24 {
			return prefix, nil
		}
		if fallback == "" {
			fallback = prefix
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("netutil: no local IPv4 address found")
}
