// Package state owns the daemon's view of read-only SQLite + Redis
// state. The daemon never writes to SQLite — that is loco-server's
// exclusive turf. Redis carries the daemon's authoritative state
// cache (`loco:state:<layoutId>:<addr>`) and the cross-process
// pub/sub channels (§7e.3).
package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-rel/rel"
	"github.com/go-rel/sqlite3"
	_ "modernc.org/sqlite"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/repo"
)

// SQLite wraps the daemon-side read-only handle to the BigFred
// database. The connection is opened with `_pragma=query_only(1)` so
// any accidental INSERT/UPDATE bombs at the driver — defence in
// depth on top of "the daemon's code never writes".
type SQLite struct {
	repo rel.Repository
	db   *sql.DB

	users           *repo.Users
	layouts         *repo.Layouts
	commandStations *repo.CommandStations
	layoutCmdStns   *repo.LayoutCommandStations
	vehicles        *repo.Vehicles
	layoutVehicles  *repo.LayoutVehicles
}

// OpenSQLite opens the database at path for read-only access. The
// underlying *sql.DB stays alive until Close.
func OpenSQLite(path string) (*SQLite, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=query_only(1)&mode=ro",
		path,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite RO: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	r := rel.New(sqlite3.New(db))
	return &SQLite{
		repo:            r,
		db:              db,
		users:           repo.NewUsers(r),
		layouts:         repo.NewLayouts(r),
		commandStations: repo.NewCommandStations(r),
		layoutCmdStns:   repo.NewLayoutCommandStations(r),
		vehicles:        repo.NewVehicles(r),
		layoutVehicles:  repo.NewLayoutVehicles(r),
	}, nil
}

// Close releases the underlying *sql.DB.
func (s *SQLite) Close() error { return s.db.Close() }

// Layout returns the daemon-bound layout row. Any error other than
// "row exists" is fatal at boot.
func (s *SQLite) Layout(ctx context.Context, layoutID uint) (domain.Layout, error) {
	return s.layouts.FindByID(ctx, layoutID)
}

// CommandStation returns the daemon-bound command-station row.
func (s *SQLite) CommandStation(ctx context.Context, commandStationID uint) (domain.CommandStation, error) {
	return s.commandStations.FindByID(ctx, commandStationID)
}

// LayoutAttached reports whether (layoutID, commandStationID) is a
// valid pair according to layout_command_stations. The daemon
// refuses to boot when this returns false — the supervisord program
// would otherwise be unreachable from any user session.
func (s *SQLite) LayoutAttached(ctx context.Context, layoutID, commandStationID uint) (bool, error) {
	_, err := s.layoutCmdStns.Find(ctx, layoutID, commandStationID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, repo.ErrLayoutCommandStationNotFound) {
		return false, nil
	}
	return false, err
}

// ListLayoutVehicleAddresses returns the DCC addresses of every
// vehicle attached to the daemon's layout. The daemon uses this set
// to (a) gate `loco.subscribe` and (b) seed the poller. Dummy
// vehicles (DCCAddress == nil) are omitted because they can never
// be driven.
func (s *SQLite) ListLayoutVehicleAddresses(ctx context.Context, layoutID uint) ([]uint16, error) {
	rosterRows, err := s.layoutVehicles.ListByLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rosterRows))
	for _, r := range rosterRows {
		ids = append(ids, r.VehicleID)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.vehicles.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	addrs := make([]uint16, 0, len(rows))
	for _, v := range rows {
		if v.DCCAddress == nil {
			continue
		}
		addrs = append(addrs, *v.DCCAddress)
	}
	return addrs, nil
}
