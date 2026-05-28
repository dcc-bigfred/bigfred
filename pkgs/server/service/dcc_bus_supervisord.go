package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/keskad/loco/pkgs/server/supervisord"
	"github.com/keskad/loco/pkgs/server/ws"
)

// CommandStationIDsForLayout resolves the command-station ids attached
// to one layout. The concrete *LayoutService satisfies this.
type CommandStationIDsForLayout interface {
	CommandStationIDsForLayout(ctx context.Context, layoutID uint) ([]uint, error)
}

// SyncProgramsForLayouts rebuilds the supervisord `dcc-bus` group so it
// contains exactly one program per (layout, commandStation) pair drawn
// from the supplied layouts. Port assignments are preserved when
// already allocated and persisted in Redis.
func (d *DccBusService) SyncProgramsForLayouts(
	ctx context.Context,
	layoutIDs []uint,
	resolve CommandStationIDsForLayout,
) error {
	if d.sup == nil {
		return nil
	}
	if len(layoutIDs) == 0 {
		if d.log != nil {
			d.log.Info("dcc-bus supervisord sync: no layouts, clearing dcc-bus group")
		}
		return d.sup.ReplaceGroupPrograms(ctx, DccBusGroupName, nil)
	}

	seen := make(map[uint]struct{}, len(layoutIDs))
	uniqueLayouts := make([]uint, 0, len(layoutIDs))
	for _, id := range layoutIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueLayouts = append(uniqueLayouts, id)
	}

	if d.log != nil {
		d.log.WithField("layoutIds", uniqueLayouts).Info("dcc-bus supervisord sync: rebuilding programs")
	}

	desiredKeys := make(map[portKey]struct{})
	programs := make([]supervisord.ProgramSpec, 0)

	for _, layoutID := range uniqueLayouts {
		csIDs, err := resolve.CommandStationIDsForLayout(ctx, layoutID)
		if err != nil {
			return err
		}
		for _, csID := range csIDs {
			if csID == 0 {
				continue
			}
			key := portKey{LayoutID: layoutID, CommandStationID: csID}
			desiredKeys[key] = struct{}{}

			port, err := d.ensurePort(ctx, layoutID, csID)
			if err != nil {
				return fmt.Errorf("dcc-bus sync layout %d cs %d: %w", layoutID, csID, err)
			}
			name := programName(layoutID, csID)
			programs = append(programs, d.buildProgramSpec(name, layoutID, csID, port))
		}
	}

	sort.Slice(programs, func(i, j int) bool {
		return programs[i].Name < programs[j].Name
	})

	pruned := d.prunePorts(ctx, desiredKeys)
	if d.log != nil && pruned > 0 {
		d.log.WithField("prunedPorts", pruned).Info("dcc-bus supervisord sync: dropped stale port assignments")
	}
	programNames := make([]string, len(programs))
	for i, p := range programs {
		programNames[i] = p.Name
	}
	if d.log != nil {
		d.log.WithFields(map[string]any{
			"programs": programNames,
			"count":    len(programs),
		}).Info("dcc-bus supervisord sync: applying dcc-bus group")
	}
	if err := d.sup.ReplaceGroupPrograms(ctx, DccBusGroupName, programs); err != nil {
		if d.log != nil {
			d.log.WithError(err).Error("dcc-bus supervisord sync: apply failed")
		}
		return err
	}
	if d.log != nil {
		d.log.WithField("count", len(programs)).Info("dcc-bus supervisord sync: complete")
	}
	return nil
}

// SyncProgramsForOnlineLayouts rebuilds supervisord for every layout
// that has at least one live WebSocket session, plus any ids listed in
// ensureLayoutIDs. The extras cover the dashboard HTTP poll racing
// ahead of the WS upgrade (same issue as presence list).
func (d *DccBusService) SyncProgramsForOnlineLayouts(
	ctx context.Context,
	hub *ws.Hub,
	resolve CommandStationIDsForLayout,
	ensureLayoutIDs ...uint,
) error {
	var layoutIDs []uint
	if hub != nil {
		layoutIDs = hub.LayoutIDsWithOnlineUsers()
	}
	layoutIDs = mergeLayoutIDs(layoutIDs, ensureLayoutIDs...)
	if d.log != nil {
		d.log.WithField("layoutIds", layoutIDs).Debug("dcc-bus supervisord sync: target layouts")
	}
	return d.SyncProgramsForLayouts(ctx, layoutIDs, resolve)
}

func mergeLayoutIDs(base []uint, extra ...uint) []uint {
	seen := make(map[uint]struct{}, len(base)+len(extra))
	out := make([]uint, 0, len(base)+len(extra))
	for _, id := range append(base, extra...) {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (d *DccBusService) ensurePort(ctx context.Context, layoutID, commandStationID uint) (uint16, error) {
	key := portKey{LayoutID: layoutID, CommandStationID: commandStationID}

	d.mu.Lock()
	port, ok := d.ports[key]
	d.mu.Unlock()
	if ok {
		return port, nil
	}

	port, err := d.allocatePortLocked(layoutID, commandStationID)
	if err != nil {
		return 0, err
	}
	d.persistPort(ctx, key, port)
	return port, nil
}

func (d *DccBusService) prunePorts(ctx context.Context, desired map[portKey]struct{}) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	pruned := 0
	for k := range d.ports {
		if _, keep := desired[k]; keep {
			continue
		}
		delete(d.ports, k)
		pruned++
		if d.redis != nil {
			field := fmt.Sprintf("%d:%d", k.LayoutID, k.CommandStationID)
			_ = d.redis.Client().HDel(ctx, portsRedisKey, field).Err()
		}
	}
	return pruned
}

// DccBusLayoutSync watches command-station attachment sets and
// triggers supervisord rebuilds when they change on the dashboard
// poll path.
type DccBusLayoutSync struct {
	dcc     *DccBusService
	layouts CommandStationIDsForLayout
	hub     *ws.Hub

	mu    sync.Mutex
	cache map[uint]string // layoutID → fingerprint of sorted cs ids
}

// NewDccBusLayoutSync returns a sync helper. Pass nil dcc to disable
// supervisord updates (e.g. --no-supervisor).
func NewDccBusLayoutSync(dcc *DccBusService, layouts CommandStationIDsForLayout, hub *ws.Hub) *DccBusLayoutSync {
	return &DccBusLayoutSync{
		dcc:     dcc,
		layouts: layouts,
		hub:     hub,
		cache:   make(map[uint]string, 8),
	}
}

func commandStationIDsFingerprint(ids []uint) string {
	if len(ids) == 0 {
		return ""
	}
	sorted := append([]uint(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	parts := make([]string, len(sorted))
	for i, id := range sorted {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, ",")
}

// ObserveLayout compares the layout's current command-station id set
// with the last seen value. When it changed, supervisord is
// regenerated for every layout that currently has online users.
func (s *DccBusLayoutSync) ObserveLayout(ctx context.Context, layoutID uint) error {
	if s == nil || s.dcc == nil || s.layouts == nil {
		return nil
	}
	ids, err := s.layouts.CommandStationIDsForLayout(ctx, layoutID)
	if err != nil {
		return err
	}
	fp := commandStationIDsFingerprint(ids)
	s.mu.Lock()
	unchanged := s.cache[layoutID] == fp
	if !unchanged {
		s.cache[layoutID] = fp
	}
	s.mu.Unlock()
	if unchanged {
		return nil
	}
	if s.dcc.log != nil {
		s.dcc.log.WithFields(map[string]any{
			"layoutId": layoutID,
			"csIds":    ids,
		}).Info("dcc-bus layout sync: command-station set changed, refreshing supervisord")
	}
	if err := s.dcc.SyncProgramsForOnlineLayouts(ctx, s.hub, s.layouts, layoutID); err != nil {
		s.dcc.log.WithError(err).WithField("layoutId", layoutID).Error("dcc-bus layout sync: supervisord refresh failed")
	}
	return nil
}
