// Package repo wires REL (Data Mapper ORM) to a SQLite database
// opened through the pure-Go `modernc.org/sqlite` driver, so the
// resulting binary still builds with CGO_ENABLED=0.
package repo

import (
	"database/sql"
	"fmt"

	"github.com/go-rel/rel"
	"github.com/go-rel/sqlite3"

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
func Open(path string) (rel.Repository, *sql.DB, error) {
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
	return rel.New(adapter), db, nil
}
