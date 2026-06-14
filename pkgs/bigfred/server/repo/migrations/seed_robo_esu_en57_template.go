package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const roboEsuEn57TemplateName = "ROBO / ESU Loksound / EN57"

type roboEsuEn57FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// roboEsuEn57Functions is the F0–F28 mapping for the ROBO ESU Loksound EN57.
var roboEsuEn57Functions = []roboEsuEn57FunctionSeed{
	{0, "Włączanie / wyłączanie świateł białych przednich i czerwonych tylnych zmiennych kierunkowo", "headlight"},
	{1, "Włączanie / wyłączanie dźwięku", "sound"},
	{2, "Trąbka wysoki ton", "horn_high"},
	{3, "Oświetlenie przedziału pasażerskiego", "interior_light"},
	{4, "Oświetlenie tablicy relacyjnej", "destination_board_lights"},
	{5, "Oświetlenie czoła pociągu do jazdy po torze w kierunku przeciwnym do zasadniczego (sygnał Pc2)", "pc2_signal"},
	{6, "Oświetlenie manewrowe (zredukowana prędkość oraz czasy rozpędzania i hamowania)", "shunting_mode"},
	{7, "Przyciemnianie świateł białych", "light"},
	{8, "Wyłączanie oświetlenia kabiny maszynisty", "cab_light"},
	{9, "Otwieranie / zamykanie drzwi wejściowych", "door"},
	{10, "Sprężarka", "compressor"},
	{11, "Podnoszenie / opuszczanie pantografu", "pantograph"},
	{12, "Trąbka dwutonowa", "horn_high"},
	{13, "Zapowiedź stacyjna #1", "speaker"},
	{14, "Zapowiedź stacyjna #2", "speaker"},
	{15, "Rozmowa przez radiotelefon #1", "radio_command"},
	{16, "Rozmowa przez radiotelefon #2", "radio_command"},
	{17, "Trąbka niski ton", "horn_low"},
	{18, "Włączenie / luzowanie hamulca", "brake_sound"},
	{19, "Wyciszenie dźwięku", "mute_sounds"},
	{20, "Gwizdek konduktora", "whistle"},
	{21, "Wyłączenie dźwięków hamowania", "brake_sound_mute"},
	{22, "Jazda po rozjazdach", "wheels"},
	{23, "Stukot kół na łączeniach szyn", "wheels"},
	{24, "Wentylator", "fan"},
	{25, "Prędkościomierz (Hasler)", "dashboard_light"},
	{26, "Wydmuch sprężonego powietrza", "steam_release"},
	{27, "Sprzęganie", "coupling"},
	{28, "Rozmowa przez radiotelefon #3", "radio_command"},
}

func seedRoboEsuEn57TemplateUp(s *rel.Schema) {
	name := sqlLiteral(roboEsuEn57TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range roboEsuEn57Functions {
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

func seedRoboEsuEn57TemplateDown(s *rel.Schema) {
	name := sqlLiteral(roboEsuEn57TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
