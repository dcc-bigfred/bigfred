package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const railboxRb23xxTp1TemplateName = "RB23xx / Tp1"

type railboxRb23xxTp1FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// railboxRb23xxTp1Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for Tp1 steam locomotives.
var railboxRb23xxTp1Functions = []railboxRb23xxTp1FunctionSeed{
	{0, "F0", "light"},
	{1, "Silnik", "engine"},
	{2, "Gwizdek długi", "horn_high"},
	{3, "Gwizdek krótki", "horn_low"},
	{5, "Światła długie", "headlight"},
	{6, "Tryb manewrowy", "shunting_mode"},
	{9, "Gwizdek", "whistle"},
	{10, "Sprzęganie", "coupling"},
	{11, "Rozsprzęganie", "uncoupling"},
	{12, "Stukot kół", "wheels"},
	{13, "Skrzypienie kół", "wheel_squeal"},
	{14, "Hamulec", "brake_sound"},
	{15, "Zapowiedź 1", "speaker"},
	{16, "Zapowiedź 2", "speaker"},
	{17, "Dzwon", "bell"},
	{18, "Para", "steam_release"},
	{19, "Węgel", "coal_bunker"},
	{20, "Nawadnianie", "watering"},
	{21, "Piaskowanie", "sander"},
	{22, "Wył. dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie", "mute_sounds"},
	{24, "Nawęglanie", "coal_shoveling"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxTp1TemplateUp(s *rel.Schema) {
	name := sqlLiteral(railboxRb23xxTp1TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range railboxRb23xxTp1Functions {
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

func seedRailboxRb23xxTp1TemplateDown(s *rel.Schema) {
	name := sqlLiteral(railboxRb23xxTp1TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
