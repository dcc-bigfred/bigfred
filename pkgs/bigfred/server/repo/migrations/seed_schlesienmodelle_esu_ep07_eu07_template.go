package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const schlesienModelleEsuEp07Eu07TemplateName = "SchlesienModelle / ESU Loksound / EP07 - EU07"

type schlesienModelleEsuEp07Eu07FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// schlesienModelleEsuEp07Eu07Functions is the F0–F27 mapping from the
// SchlesienModelle EU07 303E instruction manual (ESU LokSound V4.0).
// F28 is unassigned on this decoder.
var schlesienModelleEsuEp07Eu07Functions = []schlesienModelleEsuEp07Eu07FunctionSeed{
	{0, "Światła czołowe białe krótkie", "light"},
	{1, "Dźwięk wł. / wył.", "sound"},
	{2, "Syrena wysokotonowa", "horn_high"},
	{3, "Światła tylne czerwone", "red_lights"},
	{4, "Światła długie", "headlight"},
	{5, "Syrena niskotonowa", "horn_low"},
	{6, "Światła + jazda manewrowa", "shunting_mode"},
	{7, "Oświetlenie kabiny", "cab_light"},
	{8, "Syrena uszkodzona", "bell"},
	{9, "Jazda po torze niewłaściwym", "pc2_signal"},
	{10, "Światła postojowe", "side_lights"},
	{11, "Stukot kół", "wheels"},
	{12, "Kompresor główny", "compressor"},
	{13, "Postój na szlaku", "beacon_light"},
	{14, "Pisk kół w łukach", "wheel_squeal"},
	{15, "Kompresor pomocniczy", "compressor"},
	{16, "Jazda luzem + sprzęganie", "coupling"},
	{17, "Piasecznica", "sander"},
	{18, "Spuszczanie powietrza", "steam_release"},
	{19, "Gwizdek konduktora", "whistle"},
	{20, "Sygnał alarmowy", "danger"},
	{21, "Hamulec postojowy", "hand_brake"},
	{22, "Dźwięk zamykania drzwi kabiny", "door"},
	{23, "Zapowiedź stacyjna #1", "speaker"},
	{24, "Zapowiedź stacyjna #2", "speaker"},
	{25, "Oświetlenie pulpitu", "dashboard_light"},
	{26, "Iskrzenie odbieraka A", "pantograph"},
	{27, "Iskrzenie odbieraka B", "pantograph"},
}

func seedSchlesienModelleEsuEp07Eu07TemplateUp(s *rel.Schema) {
	name := sqlLiteral(schlesienModelleEsuEp07Eu07TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range schlesienModelleEsuEp07Eu07Functions {
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

func seedSchlesienModelleEsuEp07Eu07TemplateDown(s *rel.Schema) {
	name := sqlLiteral(schlesienModelleEsuEp07Eu07TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
