package inbound

import (
	"net"
	"strconv"
	"strings"
)

// ClientKey formats a protocol-scoped registry key: "<protocol>:<endpoint>".
func ClientKey(protocol, endpoint string) string {
	if protocol == "" {
		return endpoint
	}
	return protocol + ":" + endpoint
}

// ParseClientKey splits a registry key into protocol and endpoint.
func ParseClientKey(key string) (protocol, endpoint string) {
	if i := strings.IndexByte(key, ':'); i > 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

// EndpointFromAddr returns the session endpoint (IP:port or IP when sticky).
func EndpointFromAddr(addr *net.UDPAddr, ipStickiness bool) string {
	if addr == nil {
		return ""
	}
	if ipStickiness {
		return addr.IP.String()
	}
	return net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port))
}
