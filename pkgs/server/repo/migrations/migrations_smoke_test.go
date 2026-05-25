package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/keskad/loco/pkgs/server/repo"
)

// TestMigrateUpSmoke ensures every migration runs cleanly on a fresh
// SQLite file. A failure here usually means a typo in a CREATE TABLE
// or a CHECK constraint that REL's `Schema` builder cannot render.
func TestMigrateUpSmoke(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "smoke.db")
	t.Cleanup(func() { _ = os.Remove(path) })

	repository, sqlDB, err := repo.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	MigrateUp(context.Background(), repository)
}
