package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoSm31TemplateName = "PIKO / DCC24 / ESU / SM31"

type pikoSm31FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoSm31Functions is the F0–F31 mapping from the PIKO DCC24 ESU decoder
// leaflet for the SM31 locomotive.
var pikoSm31Functions = []pikoSm31FunctionSeed{
	{0, "Światła białe + podświetlenie pulpitu maszynisty zmienne kierunkowo + dźwięk przełącznika", "headlight"},
	{1, "Włączenie / wyłączenie dźwięku silnika spalinowego a8C22W", "engine"},
	{2, "Trąbka długa", "horn_high"},
	{3, "Trąbka krótka", "horn_low"},
	{4, "Światła czerwone zmienne kierunkowo + dźwięk przełącznika", "red_lights"},
	{5, "Oświetlenie kabiny maszynisty + dźwięk przełącznika", "cab_light"},
	{6, "Tryb jazdy manewrowej (zredukowane czasy zwalniania i przyspieszania, zredukowana prędkość) + światła manewrowe zmienne (przy wyłączonym F0 stałe tylko po prawej stronie)", "shunting_mode"},
	{7, "Tarcie kół o szyny na łukach", "wheel_squeal"},
	{8, "Światła do jazdy po torze przeciwnym do zasadniczego (sygnał Pc2)", "pc2_signal"},
	{9, "Wydmuch sprężonego powietrza", "steam_release"},
	{10, "Gwizdek konduktora", "whistle"},
	{11, "Dźwięk sprzęgania / rozsprzęgania", "coupling"},
	{12, "Światła mocne / słabe", "light"},
	{13, "Uruchomienie / zwolnienie hamulca lokomotywy", "brake_sound"},
	{14, "Zapowiedź stacyjna", "speaker"},
	{15, "Oświetlenie rewizyjne podwozia", "undercarriage_light"},
	{16, "Otwieranie / zamykanie drzwi kabiny", "door"},
	{17, "Przycisk automatycznego hamowania (po włączeniu tej funkcji model się zatrzymuje, po wyłączeniu rusza i rozpędza się do pierwotnej prędkości)", "brake_sound"},
	{18, "Klakson", "bell"},
	{19, "Hamulec ręczny", "hand_brake"},
	{20, "Stukot kół", "wheels"},
	{21, "Odgłos przejazdu przez rozjazdy", "wheels"},
	{22, "Radiotelefon #1", "radio_command"},
	{23, "Wentylator", "fan"},
	{24, "Radiotelefon #2", "radio_command"},
	{25, "Piasecznice", "sander"},
	{26, "Wyciszenie dźwięku o 50% (tryb tunelu)", "mute_sounds"},
	{27, "Wycieraczka okienna (2 tryby pracy ustawiane przez CV168, 0 lub 1)", "wipers"},
	{28, "Hamulec pociągowy", "brake_sound"},
	{29, "Pompa paliwa", "oil_pump"},
	{30, "Sprężarka (włącza się też automatycznie w losowych odstępach czasu)", "compressor"},
	{31, "Ręczne podniesienie obrotów silnika do maksimum", "volume_up"},
}

// seedPikoSm31TemplateUp inserts the PIKO / DCC24 / ESU / SM31 catalogue
// template with all 32 function slots. The owner is the bootstrap admin
// when that account already exists; otherwise owner_user_id stays 0 until
// an admin is seeded (only admins can edit owner-less catalogue rows).
func seedPikoSm31TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoSm31TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoSm31Functions {
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

func seedPikoSm31TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoSm31TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}

func sqlLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
