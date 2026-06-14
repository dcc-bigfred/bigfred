package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const rocoDcc24ZimoTy2TemplateName = "Roco / DCC24 / ZIMO / Ty2"

type rocoDcc24ZimoTy2FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// rocoDcc24ZimoTy2Functions is the F0–F23 mapping for the Roco DCC24 ZIMO Ty2.
var rocoDcc24ZimoTy2Functions = []rocoDcc24ZimoTy2FunctionSeed{
	{0, "Światła", "light"},
	{1, "Dźwięk jazdy", "sound"},
	{2, "Gwizd krótki", "horn_low"},
	{3, "Gwizd długi", "horn_high"},
	{4, "Gwizd długi 2", "horn_high"},
	{5, "Sprzęganie / rozprzęganie", "coupling"},
	{6, "Tryb manewrowy", "shunting_mode"},
	{7, "Pisk kół na zakrętach (tylko z F1 i podczas jazdy)", "wheel_squeal"},
	{8, "Pompa powietrza", "compressor"},
	{9, "Gwizdek konduktora", "whistle"},
	{10, "Światła manewrowe", "shunting_steps_light"},
	{11, "Szuflowanie węgla", "coal_shoveling"},
	{12, "Zapowiedź / komunikat", "speaker"},
	{13, "Odwadnianie (tylko gdy F1 jest włączone)", "watering"},
	{14, "Wyciszenie", "mute_sounds"},
	{15, "Odmulanie", "smoke"},
	{16, "Inżektor / wtryskiwacz", "injector"},
	{17, "Pompa zasilająca", "oil_pump"},
	{18, "Dmuchawa pomocnicza", "fan"},
	{19, "Prądnica", "engine"},
	{20, "Napełnianie wodą", "watering"},
	{21, "Piaskowanie", "sander"},
	{22, "Zwiększenie głośności", "volume_up"},
	{23, "Zmniejszenie głośności", "volume_down"},
}

func seedRocoDcc24ZimoTy2TemplateUp(s *rel.Schema) {
	name := sqlLiteral(rocoDcc24ZimoTy2TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range rocoDcc24ZimoTy2Functions {
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

func seedRocoDcc24ZimoTy2TemplateDown(s *rel.Schema) {
	name := sqlLiteral(rocoDcc24ZimoTy2TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
