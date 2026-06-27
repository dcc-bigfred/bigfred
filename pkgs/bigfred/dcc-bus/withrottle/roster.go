package withrottle

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// BuildRosterLine builds the RL roster line for one client state.
func BuildRosterLine(session *contract.RemoteSessionWire, allowedVehicles contract.AllowedVehicles, sentinelAddr uint16, paired bool) string {
	if !paired {
		return contract.FormatWithrottleSentinelRosterLine(sentinelAddr)
	}
	entries := rosterEntries(session, allowedVehicles)
	if len(entries) == 0 {
		return "RL0"
	}
	var b strings.Builder
	b.WriteString("RL")
	b.WriteString(strconv.Itoa(len(entries)))
	for _, e := range entries {
		b.WriteString(entrySep)
		b.WriteString(e.name)
		b.WriteString(segmentSep)
		b.WriteString(strconv.Itoa(int(e.addr)))
		b.WriteString(segmentSep)
		if e.isLong {
			b.WriteString("L")
		} else {
			b.WriteString("S")
		}
	}
	return b.String()
}

type rosterEntry struct {
	name   string
	addr   uint16
	isLong bool
}

func rosterEntries(session *contract.RemoteSessionWire, allowed contract.AllowedVehicles) []rosterEntry {
	if session == nil {
		return nil
	}
	if session.AllowAllVehicles {
		seen := make(map[uint16]struct{}, len(allowed.Vehicles))
		out := make([]rosterEntry, 0, len(allowed.Vehicles))
		for _, v := range allowed.Vehicles {
			if _, dup := seen[v.Addr]; dup {
				continue
			}
			seen[v.Addr] = struct{}{}
			name := v.VehicleID
			if name == "" {
				name = fmt.Sprintf("Loco %d", v.Addr)
			}
			out = append(out, rosterEntry{
				name:   name,
				addr:   v.Addr,
				isLong: v.Addr >= 128,
			})
		}
		return out
	}
	addrAllow := make(map[uint16]struct{}, len(session.AllowedAddrs))
	for _, a := range session.AllowedAddrs {
		addrAllow[a] = struct{}{}
	}
	vidAllow := make(map[string]struct{}, len(session.VehicleIDs))
	for _, id := range session.VehicleIDs {
		vidAllow[id] = struct{}{}
	}
	seen := make(map[uint16]struct{})
	var out []rosterEntry
	for _, v := range allowed.Vehicles {
		if len(vidAllow) > 0 {
			if _, ok := vidAllow[v.VehicleID]; !ok {
				continue
			}
		} else if len(addrAllow) > 0 {
			if _, ok := addrAllow[v.Addr]; !ok {
				continue
			}
		} else {
			continue
		}
		if _, dup := seen[v.Addr]; dup {
			continue
		}
		seen[v.Addr] = struct{}{}
		name := v.VehicleID
		if name == "" {
			name = fmt.Sprintf("Loco %d", v.Addr)
		}
		out = append(out, rosterEntry{
			name:   name,
			addr:   v.Addr,
			isLong: v.Addr >= 128,
		})
	}
	if len(out) == 0 && len(addrAllow) > 0 {
		for addr := range addrAllow {
			if _, dup := seen[addr]; dup {
				continue
			}
			seen[addr] = struct{}{}
			out = append(out, rosterEntry{
				name:   fmt.Sprintf("Loco %d", addr),
				addr:   addr,
				isLong: addr >= 128,
			})
		}
	}
	return out
}
