package ctl

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/version"
)

// LayoutLister is cmd.Layout.ListAll.
type LayoutLister interface {
	ListAll(ctx context.Context) ([]domain.Layout, error)
}

// StationLister is cmd.CommandStation.ListAll.
type StationLister interface {
	ListAll(ctx context.Context) ([]domain.CommandStation, error)
}

// DccBusPrograms is service.DccBusService.ProgramsForCommandStation.
type DccBusPrograms interface {
	ProgramsForCommandStation(ctx context.Context, commandStationID uint) ([]service.DccBusProgramStatus, error)
}

// Handler dispatches one request per connection (poll, not watch).
type Handler struct {
	Layouts  LayoutLister
	Stations StationLister
	DccBus   DccBusPrograms
}

// DccBusListResponse matches GET /api/v1/admin/dcc-bus/{id}/services,
// concatenated across every command station.
type DccBusListResponse struct {
	Programs []service.DccBusProgramStatus `json:"programs"`
}

type request struct {
	Type string `json:"type"`
}

// Serve accepts clients until ln is closed or ctx is done.
func Serve(ctx context.Context, ln net.Listener, h Handler, log *logrus.Logger) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	slots := make(chan struct{}, MaxClients)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		select {
		case slots <- struct{}{}:
		default:
			_ = writeError(conn, "busy")
			_ = conn.Close()
			continue
		}
		go func(c net.Conn) {
			defer func() { <-slots }()
			h.serveConn(ctx, c, log)
		}(conn)
	}
}

func (h Handler) serveConn(ctx context.Context, conn net.Conn, log *logrus.Logger) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	raw, err := ReadFrame(conn)
	if err != nil {
		return
	}
	var req request
	if err := json.Unmarshal(raw, &req); err != nil || req.Type == "" {
		_ = writeError(conn, "invalid_request")
		return
	}

	switch req.Type {
	case "version":
		_ = writeJSON(conn, version.Get())
	case "layouts_list":
		h.handleLayouts(ctx, conn, log)
	case "dcc_bus_list":
		h.handleDccBus(ctx, conn, log)
	default:
		_ = writeError(conn, "invalid_request")
	}
}

func (h Handler) handleLayouts(ctx context.Context, conn net.Conn, log *logrus.Logger) {
	if h.Layouts == nil {
		_ = writeError(conn, "internal_error")
		return
	}
	rows, err := h.Layouts.ListAll(ctx)
	if err != nil {
		if log != nil {
			log.WithError(err).Error("ctl layouts_list")
		}
		_ = writeError(conn, "internal_error")
		return
	}
	out := make([]protocol.LayoutResponse, 0, len(rows))
	for _, l := range rows {
		out = append(out, protocol.ToLayoutResponse(l))
	}
	_ = writeJSON(conn, out)
}

func (h Handler) handleDccBus(ctx context.Context, conn net.Conn, log *logrus.Logger) {
	if h.DccBus == nil || h.Stations == nil {
		_ = writeError(conn, "service_unavailable")
		return
	}
	stations, err := h.Stations.ListAll(ctx)
	if err != nil {
		if log != nil {
			log.WithError(err).Error("ctl dcc_bus_list stations")
		}
		_ = writeError(conn, "internal_error")
		return
	}
	programs := make([]service.DccBusProgramStatus, 0)
	for _, cs := range stations {
		ps, err := h.DccBus.ProgramsForCommandStation(ctx, cs.ID)
		if err != nil {
			if errors.Is(err, service.ErrServicesNotWired) {
				_ = writeError(conn, "service_unavailable")
				return
			}
			if log != nil {
				log.WithError(err).Error("ctl dcc_bus_list programs")
			}
			_ = writeError(conn, "internal_error")
			return
		}
		programs = append(programs, ps...)
	}
	_ = writeJSON(conn, DccBusListResponse{Programs: programs})
}
