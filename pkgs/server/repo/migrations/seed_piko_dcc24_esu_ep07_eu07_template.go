package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoDcc24EsuEp07Eu07TemplateName = "PIKO / DCC24 / ESU LOKSOUND / EP07 - EU07"

type pikoDcc24EsuEp07Eu07FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoDcc24EsuEp07Eu07Functions is the F0–F24 mapping from the PIKO DCC24
// ESU Loksound decoder leaflet for EP07/EU07.
var pikoDcc24EsuEp07Eu07Functions = []pikoDcc24EsuEp07Eu07FunctionSeed{
	{0, "Włączenie / wyłączenie świateł białych zmiennych kierunkowo", "headlight"},
	{1, "Uruchomienie / wyłączenie lokomotywy oraz dźwięków jazdy", "engine"},
	{2, "Trąbka wysokotonowa", "horn_high"},
	{3, "Jazda manewrowa (zredukowana prędkość oraz czasy rozpędzania i hamowania) + światła manewrowe + dźwięk rozprzęgania", "shunting_mode"},
	{4, "Światła czerwone zmienne kierunkowo", "red_lights"},
	{5, "Oświetlenie kabiny zmienne kierunkowo", "cab_light"},
	{6, "Światła długie", "light"},
	{7, "Kompresor", "compressor"},
	{8, "Trąbka niskotonowa", "horn_low"},
	{9, "Gwizdek konduktora", "whistle"},
	{10, "Zapowiedź stacyjna #1", "speaker"},
	{11, "Wyłączenie dźwięku wentylatorów chłodzenia oporników rozruchowych", "fan"},
	{12, "Uszkodzona trąbka", "bell"},
	{13, "Tarcie kół o szyny na łukach i rozjazdach", "wheel_squeal"},
	{14, "Kompresor pomocniczy", "compressor"},
	{15, "Zapowiedź stacyjna #2", "speaker"},
	{16, "Otwieranie / zamykanie drzwi", "door"},
	{17, "Włączenie / zwolnienie hamulca", "brake_sound"},
	{18, "Światła postojowe", "side_lights"},
	{19, "Piasecznica", "sander"},
	{20, "Stukot kół na łączeniach szyn", "wheels"},
	{21, "Wydmuch sprężonego powietrza", "steam_release"},
	{22, "Deaktywacja dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie dźwięku", "mute_sounds"},
	{24, "Sygnał dźwiękowy alarmowy", "danger"},
}

func seedPikoDcc24EsuEp07Eu07TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoDcc24EsuEp07Eu07TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoDcc24EsuEp07Eu07Functions {
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

func seedPikoDcc24EsuEp07Eu07TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoDcc24EsuEp07Eu07TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
