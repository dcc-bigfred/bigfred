package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const railboxRb23xxVectronTemplateName = "RB23xx / Vectron"

type railboxRb23xxVectronFunctionSeed struct {
	num  uint8
	name string
	icon string
}

// railboxRb23xxVectronFunctions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for Vectron locomotives.
var railboxRb23xxVectronFunctions = []railboxRb23xxVectronFunctionSeed{
	{0, "Światła pociągowe", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka Wysoka", "horn_high"},
	{3, "Trąbka Niska", "horn_low"},
	{4, "Wentylator", "fan"},
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
	{17, "Sprężarka", "compressor"},
	{18, "Ciśnienie", "steam_release"},
	{20, "Drzwi lokomotywy", "door"},
	{21, "Piaskowanie", "sander"},
	{22, "Wył. dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie", "mute_sounds"},
	{24, "Pantograf", "pantograph"},
	{27, "Sygnał Pc2", "pc2_signal"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxVectronTemplateUp(s *rel.Schema) {
	name := sqlLiteral(railboxRb23xxVectronTemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range railboxRb23xxVectronFunctions {
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

func seedRailboxRb23xxVectronTemplateDown(s *rel.Schema) {
	name := sqlLiteral(railboxRb23xxVectronTemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
