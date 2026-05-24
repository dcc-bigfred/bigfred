// Package migrations centralises every schema change applied to the
// BigFred database. The bootstrap intentionally relies on REL's own
// migrator (github.com/go-rel/rel/migrator) so that future milestones
// can simply Register() additional versions next to the ones declared
// here without changing the call site in `main`.
package migrations

import (
	"context"

	"github.com/go-rel/rel"
	"github.com/go-rel/rel/migrator"
)

// MigrateUp applies every Register-ed migration in ascending version
// order. The migrator skips versions already recorded in the
// `rel_schema_versions` table, so calling MigrateUp on every server
// startup is safe and cheap.
func MigrateUp(ctx context.Context, repo rel.Repository) {
	m := migrator.New(repo)
	register(&m)
	m.Migrate(ctx)
}

// register is the single place where new migrations are wired in.
// Versions MUST be monotonically increasing integers (epoch-like
// stamps are recommended so concurrent feature branches don't collide).
func register(m *migrator.Migrator) {
	m.Register(20260523_000001, createUsersUp, createUsersDown)
}

func createUsersUp(s *rel.Schema) {
	s.CreateTable("users", func(t *rel.Table) {
		t.ID("id")
		t.String("login")
		t.String("pin_hash")
		t.String("role")
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Unique([]string{"login"})
	})
}

func createUsersDown(s *rel.Schema) {
	s.DropTable("users")
}
