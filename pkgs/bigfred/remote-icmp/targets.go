package main

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/redis/go-redis/v9"
)

const clientsKeyPattern = "bigfred:remote:clients:*"

// ProbeTarget is one ICMP destination derived from a clients snapshot.
type ProbeTarget struct {
	IP               net.IP
	Protocol         string
	Login            string
	LayoutID         uint
	CommandStationID uint
}

// LoadProbeTargets SCANs Redis for handset client snapshots and returns
// unique IPs (preferring a paired client's login when several share an IP).
func LoadProbeTargets(ctx context.Context, rdb *redis.Client) ([]ProbeTarget, error) {
	var cursor uint64
	byIP := make(map[string]ProbeTarget)

	for {
		keys, next, err := rdb.Scan(ctx, cursor, clientsKeyPattern, 64).Result()
		if err != nil {
			return nil, fmt.Errorf("redis scan: %w", err)
		}
		for _, key := range keys {
			raw, err := rdb.Get(ctx, key).Bytes()
			if err == redis.Nil {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("redis get %s: %w", key, err)
			}
			snap, err := contract.UnmarshalRemoteClientsSnapshot(raw)
			if err != nil {
				continue
			}
			for _, cl := range snap.Clients {
				ip := net.ParseIP(strings.TrimSpace(cl.IP))
				if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
					continue
				}
				ip4 := ip.To4()
				if ip4 == nil {
					continue // IPv4-only probes for now
				}
				keyIP := ip4.String()
				login := cl.UserLogin
				if login == "" {
					login = "_"
				}
				proto := cl.Protocol
				if proto == "" {
					proto = "_"
				}
				cand := ProbeTarget{
					IP:               ip4,
					Protocol:         proto,
					Login:            login,
					LayoutID:         snap.LayoutID,
					CommandStationID: snap.CommandStationID,
				}
				// When several clients share an IP (NAT), prefer the paired login.
				// If multiple paired clients share an IP, the first seen wins.
				prev, ok := byIP[keyIP]
				if !ok || (prev.Login == "_" && cand.Login != "_") {
					byIP[keyIP] = cand
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	out := make([]ProbeTarget, 0, len(byIP))
	for _, t := range byIP {
		out = append(out, t)
	}
	return out, nil
}
