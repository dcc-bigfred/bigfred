package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const defaultMomentaryDurationMs = 1000

// templateFunctionSeed is one F-slot inserted by a vehicle-template seed migration.
type templateFunctionSeed struct {
	num  uint8
	name string
	icon string
}

func sqlLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func isMomentaryIcon(icon string) bool {
	return strings.Contains(icon, "horn")
}

func momentarySeedValues(icon string) (momentary int, durationMs int) {
	if isMomentaryIcon(icon) {
		return 1, defaultMomentaryDurationMs
	}
	return 0, defaultMomentaryDurationMs
}

func seedTemplateFunctions(s *rel.Schema, templateName string, functions []templateFunctionSeed) {
	name := sqlLiteral(templateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range functions {
		momentary, durationMs := momentarySeedValues(fn.icon)
		parts = append(parts, fmt.Sprintf(
			`SELECT NULL, t.id, %d, '%s', '%s', %d, %d, %d, datetime('now'), datetime('now')
			 FROM vehicle_templates t
			 WHERE t.name = '%s'
			   AND NOT EXISTS (
			     SELECT 1 FROM dcc_functions f
			     WHERE f.template_id = t.id AND f.num = %d
			   )`,
			fn.num,
			sqlLiteral(fn.name),
			fn.icon,
			fn.num,
			momentary,
			durationMs,
			name,
			fn.num,
		))
	}

	s.Exec(rel.Raw(`
		INSERT INTO dcc_functions (vehicle_id, template_id, num, name, icon, position, momentary, momentary_duration_ms, created_at, updated_at)
	` + strings.Join(parts, " UNION ALL ")))
}

func deleteTemplateSeed(s *rel.Schema, templateName string) {
	name := sqlLiteral(templateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
