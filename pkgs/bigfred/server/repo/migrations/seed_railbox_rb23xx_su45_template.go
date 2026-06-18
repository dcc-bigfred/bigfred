package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const railboxRb23xxSu45TemplateName = "RB23xx / SU45"

type railboxRb23xxSu45FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// railboxRb23xxSu45Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for SU45 locomotives.
var railboxRb23xxSu45Functions = []railboxRb23xxSu45FunctionSeed{
	{0, "F0", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka Wysoka", "horn_high"},
	{3, "Trąbka Niska", "horn_low"},
	{5, "Światła długie", "headlight"},
	{6, "Tryb manewrowy", "shunting_mode"},
	{7, "Światła tylne", "red_lights"},
	{8, "Światła kabiny", "cab_light"},
	{9, "Gwizdek", "whistle"},
	{10, "Sprzęganie", "coupling"},
	{11, "Rozsprzęganie", "uncoupling"},
	{12, "Stukot kół", "wheels"},
	{13, "Skrzypienie kół", "wheel_squeal"},
	{14, "Hamulec", "brake_sound"},
	{15, "Zapowiedź 1", "speaker"},
	{16, "Zapowiedź 2", "speaker"},
	{17, "Sprężarka", "compressor"},
	{18, "Ciśnienie", "steam_release"},
	{19, "Pompa oleju", "oil_pump"},
	{20, "Drzwi lokomotywy", "door"},
	{21, "Piaskowanie", "sander"},
	{22, "Prądnica ogrzewcza", "engine"},
	{23, "Wyciszenie", "mute_sounds"},
	{25, "Drzwi wagonu 1", "door"},
	{26, "Drzwi wagonu", "door"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxSu45TemplateUp(s *rel.Schema) {
	name := sqlLiteral(railboxRb23xxSu45TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range railboxRb23xxSu45Functions {
		parts = append(parts, fmt.Sprintf(
			`SELECT NULL, t.id, %d, '%s', '%s', %d, datetime('now'), datetime('now')
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
			name,
			fn.num,
		))
	}

	s.Exec(rel.Raw(`
		INSERT INTO dcc_functions (vehicle_id, template_id, num, name, icon, position, created_at, updated_at)
	` + strings.Join(parts, " UNION ALL ")))
}

func seedRailboxRb23xxSu45TemplateDown(s *rel.Schema) {
	name := sqlLiteral(railboxRb23xxSu45TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
