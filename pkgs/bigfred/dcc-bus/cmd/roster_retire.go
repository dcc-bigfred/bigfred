package cmd

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

func (r *Router) retireRemovedLocos(ctx context.Context, addrs []uint16) {
	if len(addrs) == 0 {
		return
	}
	for _, addr := range addrs {
		r.retireRemovedLoco(ctx, addr)
	}
	if r.hub != nil {
		r.hub.UnsubscribeAll(addrs...)
	}
	r.log.WithFields(logrus.Fields{
		"layoutId": r.layoutID,
		"addrs":    addrs,
		"count":    len(addrs),
	}).Info("dcc-bus roster removed locos retired")
}

func (r *Router) retireRemovedLoco(ctx context.Context, addr uint16) {
	if err := r.station.SetSpeed(commandstation.LocoAddr(addr), 1, true, uint8(r.speedSteps)); err != nil {
		r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus roster retire SetSpeed failed")
	}
	maxFn := service.MaxDCCFunctionNum()
	for fn := uint8(0); fn <= maxFn; fn++ {
		if err := r.station.SendFn(commandstation.MainTrackMode, commandstation.LocoAddr(addr), commandstation.FuncNum(fn), false); err != nil {
			fields := r.stationLogFields()
			fields["addr"] = addr
			fields["function"] = fn
			r.log.WithError(err).WithFields(fields).Warn("dcc-bus roster retire SendFn failed")
		}
	}
	r.cache.ClearAddr(addr)

	snap := contract.LocoStateWire{
		Address:   addr,
		Speed:     0,
		Forward:   true,
		Functions: make([]bool, maxFn+1),
		Source:    "roster_removed",
		At:        time.Now().UTC().UnixMilli(),
	}
	if r.redis != nil {
		if err := r.redis.StoreLocoCurrentState(ctx, snap, StateTTL); err != nil {
			r.log.WithError(err).WithField("addr", addr).Debug("dcc-bus roster retire redis store")
		}
	}
	service.BroadcastLocoState(ctx, r.hub, snap)
}
