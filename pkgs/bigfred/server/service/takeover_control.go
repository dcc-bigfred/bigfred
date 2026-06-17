package service

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// TakeoverControlService adapts ws.Client to cmd.TakeoverControl.
type TakeoverControlService struct {
	core *cmd.TakeoverControl
}

func NewTakeoverControlService(takeover *TakeoverService) *TakeoverControlService {
	var port cmd.TakeoverControlPort
	if takeover != nil {
		port = takeover
	}
	return &TakeoverControlService{core: cmd.NewTakeoverControl(port, TakeoverDeniedCode)}
}

func (h *TakeoverControlService) HandleEnvelope(ctx context.Context, c *ws.Client, env ws.Envelope) bool {
	return h.core.HandleEnvelope(ctx, wrapControlClient(c), cmd.ControlEnvelope{
		Type:    env.Type,
		ID:      env.ID,
		Payload: env.Payload,
	})
}
