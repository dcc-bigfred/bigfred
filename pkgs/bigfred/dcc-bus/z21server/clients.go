package z21server

import "context"

func (s *Server) noteClientActivity(ctx context.Context, client *Client) {
	if client == nil {
		return
	}
	if s.registry.IsPaired(client.Key) && s.registry.IdleBraked(client.Key) {
		s.registry.ClearIdleBraked(client.Key)
	}
	if s.coordinator != nil {
		s.coordinator.NoteActivity(ctx, client.Key)
	}
}
