package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoXpSp45Su45TemplateName = "PIKO / XP / SP45 - SU45"

type pikoXpSp45Su45FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoXpSp45Su45Functions is the F0–F22 mapping from the PIKO SmartDecoder
// XP Sound leaflet for SP/SU 45 (PKP), product #56561.
var pikoXpSp45Su45Functions = []pikoXpSp45Su45FunctionSeed{
	{0, "Światła", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka wysoki ton", "horn_high"},
	{3, "Trąbka niski ton", "horn_low"},
	{4, "Pompa paliwa", "oil_pump"},
	{5, "Zestaw manewrowy", "shunting_mode"},
	{6, "Podgrzewacz", "engine"},
	{7, "Zapowiedź stacyjna", "speaker"},
	{8, "Gwizdek konduktora", "whistle"},
	{9, "Oświetlenie kabiny 1", "cab_light"},
	{10, "Oświetlenie kabiny 2", "cab_light"},
	{11, "Radio", "radio_command"},
	{12, "Sprężone powietrze", "steam_release"},
	{13, "Sprzęganie", "coupling"},
	{14, "Sprężarka", "compressor"},
	{15, "Hamulec ręczny", "hand_brake"},
	{16, "Stukot kół na łączeniach szyn", "wheels"},
	{17, "Skrzypienie kół na łukach", "wheel_squeal"},
	{18, "Wyłączenie dźwięku hamowania", "brake_sound_mute"},
	{19, "Oświetlenie pociągu: lok pchający", "red_lights"},
	{20, "Oświetlenie pociągu: lok ciągnący", "headlight"},
	{21, "Regulacja głośności", "speaker"},
	{22, "Tryb tunelu", "mute_sounds"},
}

func seedPikoXpSp45Su45TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoXpSp45Su45TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoXpSp45Su45Functions {
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

func seedPikoXpSp45Su45TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoXpSp45Su45TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
