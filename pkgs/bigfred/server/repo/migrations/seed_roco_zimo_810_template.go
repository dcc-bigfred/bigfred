package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const rocoZimo810TemplateName = "Roco / ZIMO / 810"

type rocoZimo810FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// rocoZimo810Functions is the F-key mapping from the ZIMO sound project for
// the Roco HO ČD series 810 diesel railcar (MX64x/MX69x, sound version Roco).
// F21–F23 and F25 are unused on this decoder.
var rocoZimo810Functions = []rocoZimo810FunctionSeed{
	{0, "Światła czołowe zależne od kierunku jazdy", "headlight"},
	{1, "Czerwone światła tylne zależne od kierunku jazdy", "red_lights"},
	{2, "Światła długie zależne od kierunku jazdy", "light"},
	{3, "Tryb manewrowy", "shunting_mode"},
	{4, "Wyłączenie krzywej przyspieszania", "valve"},
	{5, "Oświetlenie wnętrza kabiny maszynisty zależne od kierunku jazdy", "cab_light"},
	{6, "Jazda bez obciążenia", "wheels"},
	{7, "Sygnał trąbkowy 1", "horn_high"},
	{8, "Dźwięk włącz / wyłącz", "sound"},
	{9, "Sygnał trąbkowy 1 – krótki", "horn_low"},
	{10, "Sygnał trąbkowy 2", "horn_high"},
	{11, "Gwizdek 1", "whistle"},
	{12, "Zapowiedź stacyjna", "speaker"},
	{13, "Ogrzewanie", "engine"},
	{14, "Otwieranie / zamykanie drzwi", "door"},
	{15, "Kompresor (dźwięk losowy)", "compressor"},
	{16, "Gwizdek konduktora", "whistle"},
	{17, "Sprzęganie", "coupling"},
	{18, "Rozsprzęganie", "uncoupling"},
	{19, "Zestaw dźwięków – z ładunkiem / bez ładunku", "sound"},
	{20, "Skrzypienie kół na łukach", "wheel_squeal"},
	{24, "Piasek", "sander"},
	{26, "Ściszenie dźwięku", "volume_down"},
	{27, "Zwiększenie głośności", "volume_up"},
	{28, "Wyciszenie", "mute_sounds"},
}

func seedRocoZimo810TemplateUp(s *rel.Schema) {
	name := sqlLiteral(rocoZimo810TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range rocoZimo810Functions {
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

func seedRocoZimo810TemplateDown(s *rel.Schema) {
	name := sqlLiteral(rocoZimo810TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
