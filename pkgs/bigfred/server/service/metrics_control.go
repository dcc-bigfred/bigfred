package service

import (
	"context"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// MetricsControlHandler wraps a control-plane dispatcher with WS command timing.
type MetricsControlHandler struct {
	inner ws.ControlHandler
	m     *metrics.Metrics
}

// NewMetricsControlHandler records per-command latency around inner.
func NewMetricsControlHandler(inner ws.ControlHandler, m *metrics.Metrics) ws.ControlHandler {
	if inner == nil || m == nil {
		return inner
	}
	return &MetricsControlHandler{inner: inner, m: m}
}

func (h *MetricsControlHandler) HandleOpened(ctx context.Context, client *ws.Client) {
	h.inner.HandleOpened(ctx, client)
}

func (h *MetricsControlHandler) HandleClosed(ctx context.Context, client *ws.Client) {
	h.inner.HandleClosed(ctx, client)
}

func (h *MetricsControlHandler) HandleEnvelope(ctx context.Context, client *ws.Client, env ws.Envelope) {
	start := time.Now()
	h.inner.HandleEnvelope(ctx, client, env)
	h.m.RecordWSCommand(client.Session().LayoutID, env.Type, metrics.OK(), time.Since(start))
}
