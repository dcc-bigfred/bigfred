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
	m.Register(20260525_000001, createLayoutsUp, createLayoutsDown)
	m.Register(20260525_000002, createInterlockingsUp, createInterlockingsDown)
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

// createLayoutsUp wires the `layouts` table that backs domain.Layout
// (§3a.1). On top of the obvious columns it installs two SQLite
// constraints that the spec calls out explicitly (§9 step 11):
//
//  1. CHECK (NOT (is_system = 1 AND locked = 1)) — the system layout
//     can never be locked, so this catches both a buggy service call
//     and a hand-rolled UPDATE.
//
//  2. UNIQUE partial index on `is_system` WHERE is_system = 1 —
//     guarantees at most one system row exists. SQLite does not
//     support partial indexes through REL's typed Schema builder,
//     hence the raw s.Exec.
//
// `name` is a plain unique constraint; the user-facing label for the
// system row is rendered through the i18n key `layout:system_default_label`,
// so the stored Name ("default") is an opaque system marker.
func createLayoutsUp(s *rel.Schema) {
	s.CreateTable("layouts", func(t *rel.Table) {
		t.ID("id")
		t.String("name")
		t.Bool("is_system", rel.Default(false))
		t.Bool("locked", rel.Default(false))
		t.Int("created_by", rel.Unsigned(true), rel.Default(0))
		t.DateTime("created_at")
		t.DateTime("updated_at")

		t.Unique([]string{"name"})
		t.Fragment("CHECK (NOT (is_system = 1 AND locked = 1))")
	})
	s.Exec(rel.Raw(`CREATE UNIQUE INDEX layouts_unique_system ON layouts(is_system) WHERE is_system = 1`))
}

func createLayoutsDown(s *rel.Schema) {
	s.DropTable("layouts")
}

func createInterlockingsUp(s *rel.Schema) {
	s.CreateTable("interlockings", func(t *rel.Table) {
		t.ID("id")
		t.String("name")
		t.Text("location")
		t.DateTime("created_at")

		t.Unique([]string{"name"})
	})

	s.CreateTable("layout_interlockings", func(t *rel.Table) {
		t.ID("id")
		t.Int("layout_id", rel.Unsigned(true))
		t.Int("interlocking_id", rel.Unsigned(true))
		t.Int("added_by_user_id", rel.Unsigned(true), rel.Default(0))
		t.DateTime("added_at")

		t.Unique([]string{"layout_id", "interlocking_id"})
	})
}

func createInterlockingsDown(s *rel.Schema) {
	s.DropTable("layout_interlockings")
	s.DropTable("interlockings")
}
