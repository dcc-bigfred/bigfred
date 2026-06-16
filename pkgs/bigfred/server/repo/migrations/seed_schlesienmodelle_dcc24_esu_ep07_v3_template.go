package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const schlesienModelleDcc24EsuEp07V3TemplateName = "SchlesienModelle / DCC24 / ESU LokSound / EP07 v3"

type schlesienModelleDcc24EsuEp07V3FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// schlesienModelleDcc24EsuEp07V3Functions is the F0–F31 mapping from the
// SchlesienModelle DCC24 ESU LokSound decoder leaflet for EP07 v3.
var schlesienModelleDcc24EsuEp07V3Functions = []schlesienModelleDcc24EsuEp07V3FunctionSeed{
	{0, "Włączenie / wyłączenie świateł białych i czerwonych zmiennych kierunkowo", "light"},
	{1, "Uruchomienie / wyłączenie lokomotywy oraz dźwięków jazdy", "engine"},
	{2, "Trąbka wysokotonowa", "horn_high"},
	{3, "Jazda manewrowa (zredukowana prędkość oraz czasy rozpędzania i hamowania) + światła manewrowe + dźwięk rozprzęgania", "shunting_mode"},
	{4, "Światła Pc2 (jazda po torze lewym w kierunku przeciwnym do zasadniczego)", "pc2_signal"},
	{5, "Oświetlenie kabiny zmienne kierunkowo", "cab_light"},
	{6, "Światła mocne / słabe", "light"},
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
	{18, "Wyłączenie czerwonych świateł (nie działa w pierwszej edycji EP07-361)", "red_lights"},
	{19, "Piasecznica", "sander"},
	{20, "Stukot kół na łączeniach szyn", "wheels"},
	{21, "Wydmuch sprężonego powietrza", "steam_release"},
	{22, "Deaktywacja dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie dźwięku", "mute_sounds"},
	{24, "Sygnał dźwiękowy alarmowy", "danger"},
	{25, "Wycieraczka okienna", "wipers"},
	{26, "Radio #1", "radio_command"},
	{27, "Radio #2", "radio_command"},
	{28, "Zapowiedź stacyjna", "speaker"},
	{29, "Tachograf - Hasler", "dashboard_light"},
	{30, "Odgłos przejazdu przez rozjazd", "wheels"},
	{31, "Hamulec ręczny", "hand_brake"},
}

func seedSchlesienModelleDcc24EsuEp07V3TemplateUp(s *rel.Schema) {
	name := sqlLiteral(schlesienModelleDcc24EsuEp07V3TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range schlesienModelleDcc24EsuEp07V3Functions {
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

func seedSchlesienModelleDcc24EsuEp07V3TemplateDown(s *rel.Schema) {
	name := sqlLiteral(schlesienModelleDcc24EsuEp07V3TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
