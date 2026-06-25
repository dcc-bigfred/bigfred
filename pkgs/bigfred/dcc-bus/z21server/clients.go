package z21server

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
)

// ClientsPublisher stores and fans out handset presence snapshots.
type ClientsPublisher interface {
	PublishZ21ClientsSnapshot(ctx context.Context, snap contract.Z21ClientsSnapshotWire) error
}

func (s *Server) noteClientActivity(client *Client) {
	if client == nil {
		return
	}
	if client.Paired != nil && client.IdleBraked {
		s.registry.ClearIdleBraked(client.Key)
		client.IdleBraked = false
	}
	s.publishClientsSnapshotThrottled(context.Background())
}

func (s *Server) publishClientsSnapshotThrottled(ctx context.Context) {
	if s.clientsPub == nil {
		return
	}
	s.clientsPubMu.Lock()
	defer s.clientsPubMu.Unlock()
	if !s.lastClientsPub.IsZero() && time.Since(s.lastClientsPub) < ClientsPublishMinInterval*time.Second {
		return
	}
	s.lastClientsPub = time.Now().UTC()
	_ = s.publishClientsSnapshot(ctx)
}

func (s *Server) publishClientsSnapshot(ctx context.Context) error {
	if s.clientsPub == nil {
		return nil
	}
	snap := s.buildClientsSnapshot()
	return s.clientsPub.PublishZ21ClientsSnapshot(ctx, snap)
}

func (s *Server) buildClientsSnapshot() contract.Z21ClientsSnapshotWire {
	now := time.Now().UTC()
	clients := s.registry.Snapshot()
	out := make([]contract.Z21ClientWire, 0, len(clients))
	for _, c := range clients {
		w := contract.Z21ClientWire{
			ClientKey:   c.Key,
			IP:          c.Addr.IP.String(),
			Port:        c.Addr.Port,
			Paired:      c.Paired != nil,
			LastSeenAt:  c.LastSeen.UTC().UnixMilli(),
			ConnectedAt: c.ConnectedAt.UTC().UnixMilli(),
			IdleBraked:  c.IdleBraked,
		}
		if c.Paired != nil {
			w.UserID = c.Paired.UserID
			if s.cfg.IPStickiness {
				w.SessionExpiresAt = c.LastSeen.UTC().Add(StickySessionIdleEvictAfter * time.Second).UnixMilli()
			}
		}
		out = append(out, w)
	}
	return contract.Z21ClientsSnapshotWire{
		LayoutID:         s.cfg.LayoutID,
		CommandStationID: s.cfg.CommandStationID,
		IPStickiness:     s.cfg.IPStickiness,
		UpdatedAt:        now.UnixMilli(),
		Clients:          out,
	}
}
