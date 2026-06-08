package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoXpEt21TemplateName = "PIKO / XP / ET21"

type pikoXpEt21FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoXpEt21Functions is the F0–F28 mapping from the PIKO ET21 PKP instruction
// sheet (SmartDecoder XP 5.1 Sound, #51607 / #56589).
var pikoXpEt21Functions = []pikoXpEt21FunctionSeed{
	{0, "Oświetlenie czoła i końca pociągu (Pc1 + Pc5)", "headlight"},
	{1, "Dźwięk silnika", "sound"},
	{2, "Sygnał dźwiękowy", "horn_high"},
	{3, "Oświetlenie czoła pociągu (Pc1)", "headlight"},
	{4, "Oświetlenie końca pociągu (Pc5)", "red_lights"},
	{5, "Oświetlenie kabiny maszynisty", "cab_light"},
	{6, "Oświetlenie rewizyjne", "inspection_light"},
	{7, "Bieg manewrowy", "shunting_mode"},
	{8, "Regulacja głośności", "speaker"},
	{9, "Sygnał Pc2 – jazda po torze niewłaściwym", "pc2_signal"},
	{10, "Oświetlenie przedziału maszynowego", "engine_room_light"},
	{11, "Światła krótkie", "light"},
	{12, "Radiotelefon #1", "radio_command"},
	{13, "Radiotelefon #2", "radio_command"},
	{14, "Hamowanie lokomotywy", "brake_sound"},
	{15, "Hamulec ręczny", "hand_brake"},
	{16, "Dźwięk drzwi do kabiny", "door"},
	{17, "Dźwięk podnoszenia / opuszczania pantografu", "pantograph"},
	{18, "Dźwięk wentylatorów", "fan"},
	{19, "Dźwięk piasecznicy", "sander"},
	{20, "Wyciszenie dźwięku", "mute_sounds"},
	{21, "Dźwięk sprężarki pomocniczej", "compressor"},
	{22, "Dźwięk sprężarki", "compressor"},
	{23, "Dźwięk otwierania / zamykania okna w kabinie", "window"},
	{24, "Dźwięk otwierania / zamykania drzwi do przedziału maszynowego", "door"},
	{25, "Zgrzyt kół na łukach", "wheel_squeal"},
	{26, "Stukot kół na łączeniach szyn", "wheels"},
	{27, "Dźwięk wypuszczania sprężonego powietrza", "steam_release"},
	{28, "Dźwięk pracy wycieraczek", "wipers"},
}

func seedPikoXpEt21TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoXpEt21TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoXpEt21Functions {
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

func seedPikoXpEt21TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoXpEt21TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
