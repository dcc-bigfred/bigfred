package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoXpSt44TemplateName = "PIKO / XP / ST44"

type pikoXpSt44FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoXpSt44Functions is the F0–F28 mapping from the PIKO SmartDecoder XP
// Sound leaflet for ST44 PKP (#56538).
var pikoXpSt44Functions = []pikoXpSt44FunctionSeed{
	{0, "Światła", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka", "horn_high"},
	{3, "Pompa paliwa", "oil_pump"},
	{4, "Oświetlenie kabiny", "cab_light"},
	{5, "Pozdrowienie maszynisty", "engineer_laugh"},
	{6, "Oświetlenie przedziału maszynowego", "engine_room_light"},
	{7, "Zestaw przełączników 1", "valve"},
	{8, "Zestaw przełączników 2", "valve"},
	{9, "Wycieraczki", "wipers"},
	{10, "Drzwi kabiny", "door"},
	{11, "Okno kabiny", "window"},
	{12, "Drzwi przedziału maszynowego", "door"},
	{13, "Przełącznik kierunku", "headlight"},
	{14, "Upust zaworu powietrznego", "valve"},
	{15, "Sprzęganie", "coupling"},
	{16, "Hamulec pociągowy", "brake_sound"},
	{17, "Hamulec awaryjny", "danger"},
	{18, "Gwizdek konduktora", "whistle"},
	{19, "Wentylator (kratka)", "fan"},
	{20, "Pedał czuwaka", "sifa"},
	{21, "Hamulec ręczny", "hand_brake"},
	{22, "Piaskowanie", "sander"},
	{23, "Skrzypienie kół na łukach", "wheel_squeal"},
	{24, "Stukot kół na łączeniach szyn", "wheels"},
	{25, "Sygnał Pc1, specjalny sygnał maszynisty", "side_lights"},
	{26, "Sygnał Pc2, jazda po torze przeciwnym", "pc2_signal"},
	{27, "Regulacja głośności", "speaker"},
	{28, "Tryb tunelu", "mute_sounds"},
}

func seedPikoXpSt44TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoXpSt44TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoXpSt44Functions {
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

func seedPikoXpSt44TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoXpSt44TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
