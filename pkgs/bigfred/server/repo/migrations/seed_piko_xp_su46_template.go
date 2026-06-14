package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoXpSu46TemplateName = "PIKO / XP / SU46"

type pikoXpSu46FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoXpSu46Functions is the F0–F26 mapping from the PIKO XP SU46 decoder
// leaflet. F27 and F28 are unused on this decoder.
var pikoXpSu46Functions = []pikoXpSu46FunctionSeed{
	{0, "Światła", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka wysoki ton", "horn_high"},
	{3, "Trąbka niski ton", "horn_low"},
	{4, "Oświetlenie kabiny", "cab_light"},
	{5, "Pozdrowienie maszynisty", "engineer_laugh"},
	{6, "Oświetlenie przedziału maszynowego", "engine_room_light"},
	{7, "Zestaw przełączników 1", "valve"},
	{8, "Zestaw przełączników 2", "valve"},
	{9, "Główny wyłącznik baterii", "engine"},
	{10, "Drzwi kabiny", "door"},
	{11, "Drzwi przedziału maszynowego", "door"},
	{12, "Okno kabiny", "window"},
	{13, "Okno boczne", "window"},
	{14, "Upust zaworu powietrznego", "valve"},
	{15, "Sprzęganie", "coupling"},
	{16, "Gwizdek konduktora", "whistle"},
	{17, "Sprężarka", "compressor"},
	{18, "Radio", "radio_command"},
	{19, "Hamulec ręczny", "hand_brake"},
	{20, "Piaskowanie", "sander"},
	{21, "Skrzypienie kół na łukach", "wheel_squeal"},
	{22, "Stukot kół na łączeniach szyn", "wheels"},
	{23, "Sygnał Pc1, specjalny sygnał maszynisty", "side_lights"},
	{24, "Sygnał Pc2, jazda po torze przeciwnym", "pc2_signal"},
	{25, "Regulacja głośności", "speaker"},
	{26, "Tryb tunelu", "mute_sounds"},
}

func seedPikoXpSu46TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoXpSu46TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoXpSu46Functions {
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

func seedPikoXpSu46TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoXpSu46TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
