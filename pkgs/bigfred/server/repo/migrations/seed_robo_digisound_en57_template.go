package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const roboDigisoundEn57TemplateName = "ROBO / Digisound / EN57"

type roboDigisoundEn57FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// roboDigisoundEn57Functions is the F0–F26 mapping for the ROBO Digisound EN57.
var roboDigisoundEn57Functions = []roboDigisoundEn57FunctionSeed{
	{0, "Włączanie / wyłączanie świateł białych przednich i czerwonych tylnych zmiennych kierunkowo", "headlight"},
	{1, "Włączanie / wyłączanie dźwięku", "sound"},
	{2, "Trąbka wysoki ton", "horn_high"},
	{3, "Oświetlenie przedziału pasażerskiego", "interior_light"},
	{4, "Oświetlenie tablicy relacyjnej", "destination_board_lights"},
	{5, "Oświetlenie czoła pociągu do jazdy po torze w kierunku przeciwnym do zasadniczego (sygnał Pc2)", "pc2_signal"},
	{6, "Oświetlenie manewrowe (zredukowana prędkość oraz czasy rozpędzania i hamowania)", "shunting_mode"},
	{7, "Przyciemnianie świateł białych", "light"},
	{8, "Oświetlenie kabiny maszynisty", "cab_light"},
	{9, "Otwieranie / zamykanie drzwi wejściowych", "door"},
	{10, "Sprężarka", "compressor"},
	{11, "Trąbka krótki dźwięk", "horn_low"},
	{12, "Zapowiedź stacyjna #1", "speaker"},
	{13, "Zapowiedź stacyjna #2", "speaker"},
	{14, "Rozmowa przez radiotelefon #1", "radio_command"},
	{15, "Rozmowa przez radiotelefon #2", "radio_command"},
	{16, "Trąbka niski ton", "horn_low"},
	{17, "Hamowanie / luzowanie hamulca", "brake_sound"},
	{18, "Gwizdek konduktora", "whistle"},
	{19, "Otwieranie / zamykanie drzwi w przedziale pasażerskim", "door"},
	{20, "Dźwięk Haslera (prędkościomierza) i nastawnika w kabinie maszynisty (działa przy włączonym F1)", "dashboard_light"},
	{21, "Wycieraczki", "wipers"},
	{22, "Zgrzyt kół na rozjazdach", "wheel_squeal"},
	{23, "Stukot kół na łączeniach szyn", "wheels"},
	{24, "Hamulec ręczny", "hand_brake"},
	{25, "Wyciszenie dźwięku", "mute_sounds"},
	{26, "Zmiana głośności dźwięków", "speaker"},
}

func seedRoboDigisoundEn57TemplateUp(s *rel.Schema) {
	name := sqlLiteral(roboDigisoundEn57TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range roboDigisoundEn57Functions {
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

func seedRoboDigisoundEn57TemplateDown(s *rel.Schema) {
	name := sqlLiteral(roboDigisoundEn57TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
