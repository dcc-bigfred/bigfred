// Package repo wires REL (Data Mapper ORM) to a SQLite database
// opened through the pure-Go `modernc.org/sqlite` driver, so the
// resulting binary still builds with CGO_ENABLED=0.
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/go-rel/rel"
	"github.com/go-rel/sqlite3"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/server/metrics"

	// modernc.org/sqlite registers itself under the driver name
	// "sqlite" in its init(). The go-rel/sqlite3 adapter is
	// dialect-only and accepts an already-opened *sql.DB via New(),
	// so we never use its driver-aware Open() helper.
	_ "modernc.org/sqlite"
)

// Open initialises an in-process SQLite database file and returns a
// REL repository plus the underlying *sql.DB (kept so the caller can
// Close() it on shutdown).
//
// `path` should be a filesystem path. SQLite-specific pragmas are
// appended via the DSN query string to enable WAL journaling and
// foreign-key enforcement.
//
// When log is non-nil and its level is debug or lower, every SQL query
// is logged through logrus. At info and above, go-rel instrumentation
// is disabled so adapter-query lines do not clutter production logs.
// When m is non-nil, query latency is also exported via OpenTelemetry.
func Open(path string, log *logrus.Logger, m *metrics.Metrics) (rel.Repository, *sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping sqlite: %w", err)
	}

	adapter := sqlite3.New(db)
	repository := rel.New(adapter)
	if m != nil {
		repository.Instrumentation(sqlMetricsInstrumenter(m))
	} else if log != nil && log.IsLevelEnabled(logrus.DebugLevel) {
		repository.Instrumentation(sqlInstrumenter(log))
	} else {
		repository.Instrumentation(nil)
	}
	return repository, db, nil
}

func sqlInstrumenter(log *logrus.Logger) rel.Instrumenter {
	return func(ctx context.Context, op, message string, args ...any) func(err error) {
		if strings.HasPrefix(op, "rel-") {
			return func(error) {}
		}

		start := time.Now()
		return func(err error) {
			fields := logrus.Fields{
				"duration": time.Since(start),
				"op":       op,
			}
			entry := log.WithFields(fields)
			if err != nil {
				entry.WithError(err).Debug(message)
			} else {
				entry.Debug(message)
			}
		}
	}
}

func sqlMetricsInstrumenter(m *metrics.Metrics) rel.Instrumenter {
	return func(ctx context.Context, op, message string, args ...any) func(err error) {
		if strings.HasPrefix(op, "rel-") {
			return func(error) {}
		}
		start := time.Now()
		return func(err error) {
			m.RecordDBQuery(op, time.Since(start), err)
		}
	}
}
