package z21server

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
)

// ClientsPublisher stores and fans out handset presence snapshots.
type ClientsPublisher = remotes.ClientsSnapshotPublisher

func (s *Server) noteClientActivity(ctx context.Context, client *Client) {
	if client == nil {
		return
	}
	if s.registry.IsPaired(client.Key) && s.registry.IdleBraked(client.Key) {
		s.registry.ClearIdleBraked(client.Key)
	}
	s.publishClientsSnapshotThrottled(ctx)
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
	return s.clientsPub.PublishClientsSnapshot(ctx, snap)
}

func (s *Server) buildClientsSnapshot() contract.RemoteClientsSnapshotWire {
	now := time.Now().UTC()
	clients := s.registry.Snapshot()
	out := make([]contract.RemoteClientWire, 0, len(clients))
	for _, c := range clients {
		w := contract.RemoteClientWire{
			ClientKey:   c.Key,
			IP:          c.Addr.IP.String(),
			Port:        c.Addr.Port,
			Paired:      c.Paired != nil,
			LastSeenAt:  c.LastSeen.UnixMilli(),
			ConnectedAt: c.ConnectedAt.UnixMilli(),
			IdleBraked:  c.IdleBraked,
		}
		if c.Paired != nil {
			w.UserID = c.Paired.UserID
		}
		out = append(out, w)
	}
	return contract.RemoteClientsSnapshotWire{
		LayoutID:         s.cfg.LayoutID,
		CommandStationID: s.cfg.CommandStationID,
		Protocol:         contract.RemoteProtocolZ21,
		IPStickiness:     s.cfg.IPStickiness,
		UpdatedAt:        now.UnixMilli(),
		Clients:          out,
	}
}
