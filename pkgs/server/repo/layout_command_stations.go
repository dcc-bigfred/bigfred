package repo

import (
	"context"
	"errors"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/sort"
	"github.com/go-rel/rel/where"

	"github.com/keskad/loco/pkgs/server/domain"
)

// ErrLayoutCommandStationNotFound is returned when a layout/cs join
// row cannot be located.
var ErrLayoutCommandStationNotFound = errors.New("layout command station not found")

// ErrLayoutCommandStationDuplicate is returned by Attach when the
// (layout_id, command_station_id) pair already exists. The unique
// index on the table backs the guarantee.
var ErrLayoutCommandStationDuplicate = errors.New("layout command station already attached")

// LayoutCommandStations is the persistence adapter for
// domain.LayoutCommandStation.
type LayoutCommandStations struct {
	repo rel.Repository
}

// NewLayoutCommandStations returns a LayoutCommandStations repository
// bound to r.
func NewLayoutCommandStations(r rel.Repository) *LayoutCommandStations {
	return &LayoutCommandStations{repo: r}
}

// ListByLayout returns every join row for a layout, ordered by
// `added_at` so the UI renders attachments in chronological order.
func (l *LayoutCommandStations) ListByLayout(ctx context.Context, layoutID uint) ([]domain.LayoutCommandStation, error) {
	var rows []domain.LayoutCommandStation
	err := l.repo.FindAll(ctx, &rows,
		where.Eq("layout_id", layoutID),
		sort.Asc("added_at"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByCommandStation returns every layout the command station is
// attached to. Used by hot-reload code that broadcasts roster
// invalidation events to running daemons (§7e.3).
func (l *LayoutCommandStations) ListByCommandStation(ctx context.Context, commandStationID uint) ([]domain.LayoutCommandStation, error) {
	var rows []domain.LayoutCommandStation
	err := l.repo.FindAll(ctx, &rows,
		where.Eq("command_station_id", commandStationID),
		sort.Asc("layout_id"),
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Find returns the join row for (layoutID, commandStationID) or
// ErrLayoutCommandStationNotFound when the pair is not attached.
func (l *LayoutCommandStations) Find(ctx context.Context, layoutID, commandStationID uint) (domain.LayoutCommandStation, error) {
	var row domain.LayoutCommandStation
	err := l.repo.Find(ctx, &row,
		where.Eq("layout_id", layoutID),
		where.Eq("command_station_id", commandStationID),
	)
	if err != nil {
		if errors.Is(err, rel.ErrNotFound) {
			return domain.LayoutCommandStation{}, ErrLayoutCommandStationNotFound
		}
		return domain.LayoutCommandStation{}, err
	}
	return row, nil
}

// Attach inserts a new (layout, command station) pair. Caller fills
// AddedByUserID; AddedAt defaults to time.Now().UTC() when zero.
func (l *LayoutCommandStations) Attach(ctx context.Context, row *domain.LayoutCommandStation) error {
	if row.AddedAt.IsZero() {
		row.AddedAt = time.Now().UTC()
	}
	if err := l.repo.Insert(ctx, row); err != nil {
		// REL surfaces SQLite uniqueness as a generic error; the
		// service layer already calls Find before Attach so a
		// duplicate at this point is a race. Surface a dedicated
		// sentinel so callers can decide whether to retry or 409.
		return ErrLayoutCommandStationDuplicate
	}
	return nil
}

// Detach removes the join row for (layoutID, commandStationID).
// Returns ErrLayoutCommandStationNotFound when the pair is not
// attached.
func (l *LayoutCommandStations) Detach(ctx context.Context, layoutID, commandStationID uint) error {
	row, err := l.Find(ctx, layoutID, commandStationID)
	if err != nil {
		return err
	}
	return l.repo.Delete(ctx, &row)
}

// CommandStationIDsForLayout returns command-station ids attached to a
// layout, in chronological attachment order.
func (l *LayoutCommandStations) CommandStationIDsForLayout(ctx context.Context, layoutID uint) ([]uint, error) {
	rows, err := l.ListByLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.CommandStationID)
	}
	return ids, nil
}

// DeleteAllForLayout removes every join row for a layout.
func (l *LayoutCommandStations) DeleteAllForLayout(ctx context.Context, layoutID uint) error {
	rows, err := l.ListByLayout(ctx, layoutID)
	if err != nil {
		return err
	}
	for i := range rows {
		if err := l.repo.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}

// DeleteAllForCommandStation removes every join row pointing at a
// command station (used when the catalogue row is deleted).
func (l *LayoutCommandStations) DeleteAllForCommandStation(ctx context.Context, commandStationID uint) error {
	rows, err := l.ListByCommandStation(ctx, commandStationID)
	if err != nil {
		return err
	}
	for i := range rows {
		if err := l.repo.Delete(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}

// CountLayoutsWithOnlyStation counts non-system layouts where
// commandStationID is the sole attached station. Used before deleting
// a catalogue row (§4.1).
func (l *LayoutCommandStations) CountLayoutsWithOnlyStation(ctx context.Context, commandStationID uint, layouts *Layouts) (int, error) {
	joins, err := l.ListByCommandStation(ctx, commandStationID)
	if err != nil {
		return 0, err
	}
	atRisk := 0
	for _, join := range joins {
		layout, err := layouts.FindByID(ctx, join.LayoutID)
		if err != nil {
			return 0, err
		}
		if layout.IsSystem {
			continue
		}
		attached, err := l.ListByLayout(ctx, join.LayoutID)
		if err != nil {
			return 0, err
		}
		if len(attached) == 1 {
			atRisk++
		}
	}
	return atRisk, nil
}
