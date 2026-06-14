package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoZimoSm31TemplateName = "PIKO / DCC24 / ZIMO / SM31"

type pikoZimoSm31FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoZimoSm31Functions is the F0–F28 mapping from the PIKO DCC24 ZIMO
// decoder leaflet for the SM31 locomotive.
var pikoZimoSm31Functions = []pikoZimoSm31FunctionSeed{
	{0, "Światła białe + podświetlenie pulpitu maszynisty zmienne kierunkowo + dźwięk przełącznika", "headlight"},
	{1, "Włączenie / wyłączenie dźwięku silnika spalinowego a8C22W", "engine"},
	{2, "Trąbka długa", "horn_high"},
	{3, "Trąbka krótka", "horn_low"},
	{4, "Światła czerwone zmienne kierunkowo + dźwięk przełącznika", "red_lights"},
	{5, "Oświetlenie kabiny maszynisty + dźwięk przełącznika", "cab_light"},
	{6, "Tryb jazdy manewrowej (zredukowane czasy zwalniania i przyspieszania, zredukowana prędkość) + światła manewrowe zmienne + dźwięk sprzęgania / rozsprzęgania", "shunting_mode"},
	{7, "Tarcie kół o szyny na łukach", "wheel_squeal"},
	{8, "Światła do jazdy po torze przeciwnym do zasadniczego (sygnał Pc2)", "pc2_signal"},
	{9, "Wydmuch sprężonego powietrza", "steam_release"},
	{10, "Gwizdek konduktora", "whistle"},
	{11, "Wentylator", "fan"},
	{12, "Światła mocne / słabe", "light"},
	{13, "Sprężarka (włącza się też automatycznie w losowych odstępach czasu)", "compressor"},
	{14, "Zapowiedź stacyjna", "speaker"},
	{15, "Oświetlenie rewizyjne podwozia", "undercarriage_light"},
	{16, "Otwieranie / zamykanie drzwi kabiny", "door"},
	{17, "Ręczne podniesienie obrotów silnika do maksimum", "volume_up"},
	{18, "Klakson", "bell"},
	{19, "Pompa paliwa", "oil_pump"},
	{20, "Podgrzewacz (Webasto)", "engine"},
	{21, "Radiotelefon #1", "radio_command"},
	{22, "Radiotelefon #2", "radio_command"},
	{23, "Czuwak aktywny", "sifa"},
	{24, "Tachograf (Hasler)", "dashboard_light"},
	{25, "Piasecznice", "sander"},
	{26, "Wyciszenie dźwięku (tryb tunelu)", "mute_sounds"},
	{27, "Zmniejszenie głośności (Vol-)", "volume_down"},
	{28, "Zwiększenie głośności (Vol+)", "volume_up"},
}

func seedPikoZimoSm31TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoZimoSm31TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoZimoSm31Functions {
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

func seedPikoZimoSm31TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoZimoSm31TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
