package cmd

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	buserrors "github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
)

// HandleLocoSelect leases the command-station slot for the session's active
// drive target. The WS layer runs this in a per-request goroutine so the sync
// ack (D12) does not block the read loop during a cold-path AcquireSlot.
func (r *Router) HandleLocoSelect(ctx context.Context, actor Actor, resp Responder, p protocol.LocoSelectPayload, _ string) Result {
	if !r.roster.IsOnLayout(p.Address) {
		_ = resp.SendLocoError(ctx, p.Address, security.ReasonVehicleNotOnLayout, "")
		return FailResult(security.ReasonVehicleNotOnLayout)
	}
	if r.leaser == nil {
		return OKResult()
	}

	prev := resp.SelectedAddr()
	if prev != 0 && prev != p.Address {
		// Switcher change: defer the previous slot's release by SwitcherGrace so
		// a quick A→B→A switch reuses A's slot instead of re-acquiring it.
		r.leaser.DeselectDeferred(actor.UserID, actor.SessionID, prev)
		resp.ClearSelected()
	}

	evicted, err := r.leaser.Select(actor.UserID, actor.SessionID, "ws", p.Address)
	if err != nil {
		res := leaseErrorResult(actor.UserID, r.leaser, err)
		if res.Code == buserrors.CodeVehicleCapExceeded {
			_ = resp.SendLocoErrorPayload(ctx, protocol.LocoErrorPayload{
				Address:     p.Address,
				Code:        res.Code,
				DrivenAddrs: res.DrivenAddrs,
			})
		} else {
			_ = resp.SendLocoError(ctx, p.Address, res.Code, err.Error())
		}
		return res
	}

	resp.SetSelected(p.Address)
	r.log.WithFields(logrus.Fields{
		"sessionId": actor.SessionID,
		"userId":    actor.UserID,
		"addr":      p.Address,
		"evicted":   evicted,
	}).Debug("dcc-bus loco.select")

	res := OKResult()
	res.EvictedAddr = evicted
	return res
}

// HandleLocoDeselect drops the drive target and releases the slot when this
// session was the last driver.
func (r *Router) HandleLocoDeselect(ctx context.Context, actor Actor, resp Responder, p protocol.LocoDeselectPayload, _ string) Result {
	if r.leaser != nil {
		r.leaser.Deselect(actor.UserID, actor.SessionID, p.Address)
	}
	if resp.SelectedAddr() == p.Address {
		resp.ClearSelected()
	}
	r.log.WithFields(logrus.Fields{
		"sessionId": actor.SessionID,
		"addr":      p.Address,
	}).Debug("dcc-bus loco.deselect")
	return OKResult()
}

// HandleTrainSelect leases slots for every powered member of a train.
func (r *Router) HandleTrainSelect(ctx context.Context, actor Actor, resp Responder, p protocol.TrainSelectPayload, _ string) Result {
	train, known := r.findDefinedTrain(p.TrainID)
	if d := r.trainPolicy.CanDriveTrain(actor.UserID, train, known); !d.Allowed {
		return FailResult(d.Reason)
	}
	if d := r.trainPolicy.CanDriveTrainMembers(train); !d.Allowed {
		return FailResult(d.Reason)
	}
	addrs := poweredTrainAddrs(train)
	if len(addrs) == 0 {
		return FailResult(buserrors.CodeTrainNoPoweredMembers)
	}
	if r.leaser == nil {
		return OKResult()
	}

	if err := r.leaser.SelectTrain(actor.UserID, actor.SessionID, "ws", p.TrainID, addrs); err != nil {
		res := leaseErrorResult(actor.UserID, r.leaser, err)
		if res.Code == buserrors.CodeVehicleCapExceeded {
			_ = resp.SendLocoErrorPayload(ctx, protocol.LocoErrorPayload{
				Code:        res.Code,
				DrivenAddrs: res.DrivenAddrs,
			})
		} else {
			_ = resp.SendLocoError(ctx, 0, res.Code, err.Error())
		}
		return res
	}

	r.log.WithFields(logrus.Fields{
		"sessionId": actor.SessionID,
		"trainId":   p.TrainID,
		"addrs":     addrs,
	}).Debug("dcc-bus train.select")
	return OKResult()
}

func poweredTrainAddrs(train contract.DefinedTrain) []uint16 {
	out := make([]uint16, 0, len(train.Members))
	for _, m := range train.Members {
		if m.Addr == nil || m.ExcludeFromSpeed {
			continue
		}
		out = append(out, *m.Addr)
	}
	return out
}
