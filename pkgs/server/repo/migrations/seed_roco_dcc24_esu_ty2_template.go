package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const rocoDcc24EsuTy2TemplateName = "Roco / DCC24 / ESU Loksound / Ty2"

type rocoDcc24EsuTy2FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// rocoDcc24EsuTy2Functions is the F0–F31 mapping for the Roco DCC24 ESU Loksound Ty2.
var rocoDcc24EsuTy2Functions = []rocoDcc24EsuTy2FunctionSeed{
	{0, "Światła + dźwięk generatora", "light"},
	{1, "Dźwięk silników parowych", "engine"},
	{2, "Gwizdek #1", "horn_low"},
	{3, "Gwizdek #2", "horn_high"},
	{4, "Szuflowanie węgla", "coal_shoveling"},
	{5, "Jazda z ciężkim składem (bardziej intensywne wydmuchy pary)", "heavy_load"},
	{6, "Tryb jazdy manewrowej + światła manewrowe (2 białe z przodu + 2 białe z tyłu)", "shunting_mode"},
	{7, "Pisk obręczy kół", "wheel_squeal"},
	{8, "Zapowiedź stacyjna", "speaker"},
	{9, "Wydmuch pary z cylindrów", "steam_release"},
	{10, "Gwizdek konduktora", "whistle"},
	{11, "Dźwięk sprzęgania / rozprzęgania", "coupling"},
	{12, "Jazda na wybiegu", "wheels"},
	{13, "Zwolnienie hamulca", "brake_sound"},
	{14, "Zapowiedź stacyjna #1", "speaker"},
	{15, "Generator dymu (jeśli zainstalowano)", "smoke"},
	{16, "Zawór bezpieczeństwa", "valve"},
	{17, "Przycisk automatycznego hamowania (po włączeniu tej funkcji lokomotywa się zatrzymuje, po wyłączeniu rusza i rozpędza się do pierwotnej prędkości)", "brake_sound"},
	{18, "Zapowiedź stacyjna #2", "speaker"},
	{19, "Zapowiedź stacyjna", "speaker"},
	{20, "Inżektor #1", "injector"},
	{21, "Inżektor #2", "injector"},
	{22, "Zrzut z popielnika", "coal_bunker"},
	{23, "Wyłączenie turbogeneratora", "engine"},
	{24, "Sprężarka (wolna praca)", "compressor"},
	{25, "Piasecznica", "sander"},
	{26, "Wyciszenie dźwięku o 50%", "mute_sounds"},
	{27, "Stukot kół na łączeniach szyn", "wheels"},
	{28, "Alternatywna praca silników parowych", "firebox"},
	{29, "Sprężarka (szybka praca)", "compressor"},
	{30, "Przejazd przez rozjazd", "wheels"},
	{31, "Deaktywacja dźwięku hamowania", "brake_sound_mute"},
}

func seedRocoDcc24EsuTy2TemplateUp(s *rel.Schema) {
	name := sqlLiteral(rocoDcc24EsuTy2TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range rocoDcc24EsuTy2Functions {
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

func seedRocoDcc24EsuTy2TemplateDown(s *rel.Schema) {
	name := sqlLiteral(rocoDcc24EsuTy2TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
