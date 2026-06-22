package service

import (
	"context"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// RadioControlService adapts ws.Client to cmd.RadioControl.
type RadioControlService struct {
	core *cmd.RadioControl
}

func NewRadioControlService(radio *RadioService) *RadioControlService {
	var port cmd.RadioControlPort
	if radio != nil {
		port = radioControlPort{radio: radio}
	}
	return &RadioControlService{core: cmd.NewRadioControl(port, RadioDeniedCode)}
}

func (h *RadioControlService) HandleEnvelope(ctx context.Context, c *ws.Client, env ws.Envelope) bool {
	return h.core.HandleEnvelope(ctx, wrapControlClient(c), cmd.ControlEnvelope{
		Type:    env.Type,
		ID:      env.ID,
		Payload: env.Payload,
	})
}

type radioControlPort struct {
	radio *RadioService
}

func (p radioControlPort) Send(ctx context.Context, in cmd.RadioSendInput) (domain.RadioMessage, error) {
	return p.radio.Send(ctx, SendInput{
		LayoutID:         in.LayoutID,
		FromUserID:       in.FromUserID,
		FromLogin:        in.FromLogin,
		FromOrganization: in.FromOrganization,
		ToUserID:         in.ToUserID,
		ToInterlockingID: in.ToInterlockingID,
		ContextVehicleID: in.ContextVehicleID,
		ContextTrainID:   in.ContextTrainID,
		Phrase:           in.Phrase,
		Note:             in.Note,
	})
}

func (p radioControlPort) ReplayInterlocking(ctx context.Context, layoutID, interlockingID, callerUserID uint, limit int) ([]domain.RadioMessage, error) {
	return p.radio.ReplayInterlocking(ctx, layoutID, interlockingID, callerUserID, limit)
}

func (p radioControlPort) ReplayUser(ctx context.Context, layoutID, userID uint, limit int) ([]domain.RadioMessage, error) {
	return p.radio.ReplayUser(ctx, layoutID, userID, limit)
}

func (p radioControlPort) ReplayLimit() int { return p.radio.ReplayLimit() }
