package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoDcc24EsuSp45Su45TemplateName = "PIKO / DCC24 / ESU Loksound / SP45 - SU45"

type pikoDcc24EsuSp45Su45FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoDcc24EsuSp45Su45Functions is the F0–F24 mapping from the PIKO DCC24
// ESU Loksound decoder leaflet for SP45/SU45.
var pikoDcc24EsuSp45Su45Functions = []pikoDcc24EsuSp45Su45FunctionSeed{
	{0, "Włączenie / wyłączenie świateł białych zmiennych kierunkowo", "headlight"},
	{1, "Uruchomienie / wyłączenie lokomotywy oraz dźwięków jazdy", "engine"},
	{2, "Trąbka krótka", "horn_low"},
	{3, "Jazda manewrowa (zredukowana prędkość oraz czasy rozpędzania i hamowania) + światła manewrowe + dźwięk rozprzęgania", "shunting_mode"},
	{4, "Światła czerwone zmienne kierunkowo", "red_lights"},
	{5, "Oświetlenie kabiny zmienne kierunkowo", "cab_light"},
	{6, "Światła długie", "light"},
	{7, "Dźwięk wentylatorów", "fan"},
	{8, "Trąbka długa", "horn_high"},
	{9, "Gwizdek konduktora", "whistle"},
	{10, "Zapowiedź stacyjna #1", "speaker"},
	{11, "Załączone ogrzewanie elektryczne – podniesienie obrotów na postoju", "engine"},
	{12, "Trąbka – inny ton", "bell"},
	{13, "Tarcie kół o szyny na łukach i rozjazdach", "wheel_squeal"},
	{14, "Zapowiedź stacyjna #2", "speaker"},
	{15, "Wydmuch powietrza", "steam_release"},
	{16, "Otwieranie / zamykanie drzwi", "door"},
	{17, "Włączenie / luzowanie hamulca", "brake_sound"},
	{18, "Światła postojowe", "side_lights"},
	{19, "Piasecznica", "sander"},
	{20, "Stukot kół na łączeniach szyn", "wheels"},
	{21, "Wyciszenie dźwięków do 50%", "mute_sounds"},
	{22, "Deaktywacja dźwięku hamowania", "brake_sound_mute"},
	{23, "Deaktywacja turbosprężarki", "compressor"},
	{24, "Klakson", "bell"},
}

func seedPikoDcc24EsuSp45Su45TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoDcc24EsuSp45Su45TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoDcc24EsuSp45Su45Functions {
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

func seedPikoDcc24EsuSp45Su45TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoDcc24EsuSp45Su45TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
