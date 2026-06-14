package migrations

import (
	"fmt"
	"strings"

	"github.com/go-rel/rel"
)

const pikoXpEn57TemplateName = "PIKO / XP / EN57"

type pikoXpEn57FunctionSeed struct {
	num  uint8
	name string
	icon string
}

// pikoXpEn57Functions is the F0–F28 mapping from the PIKO SmartDecoder XP
// Sound leaflet for EN57 PKP (#56599).
var pikoXpEn57Functions = []pikoXpEn57FunctionSeed{
	{0, "Światła", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka dwutonowa", "horn_high"},
	{3, "Oświetlenie kabiny", "cab_light"},
	{4, "Oświetlenie wnętrza", "interior_light"},
	{5, "Tablica kierunku", "destination_board_lights"},
	{6, "Ściemniacz reflektorów", "light"},
	{7, "Zestaw manewrowy", "shunting_mode"},
	{8, "Sygnał jazdy po torze niewłaściwym", "pc2_signal"},
	{9, "Podwójna trakcja FS 1", "engine"},
	{10, "Podwójna trakcja FS 2", "engine"},
	{11, "Regulacja głośności", "speaker"},
	{12, "Tryb tunelu", "mute_sounds"},
	{13, "Trąbka wysoki ton", "horn_high"},
	{14, "Trąbka niski ton", "horn_low"},
	{15, "Drzwi", "door"},
	{16, "Drzwi przejściowe", "door"},
	{17, "Zapowiedź pociągu #1", "speaker"},
	{18, "Zapowiedź pociągu #2", "speaker"},
	{19, "Hamulce pociągu", "brake_sound"},
	{20, "Okno pasażerskie", "window"},
	{21, "Pantograf", "pantograph"},
	{22, "Okno kabiny", "window"},
	{23, "Drzwi kabiny", "door"},
	{24, "Sprężarka powietrza pomocnicza", "compressor"},
	{25, "Sprężarka", "compressor"},
	{26, "Sprzęganie", "coupling"},
	{27, "Wyłączenie wentylatora", "fan"},
	{28, "Hamulec pneumatyczny", "brake_sound"},
}

func seedPikoXpEn57TemplateUp(s *rel.Schema) {
	name := sqlLiteral(pikoXpEn57TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		INSERT INTO vehicle_templates (name, description, owner_user_id, version, created_at, updated_at)
		SELECT '%s', '', COALESCE((SELECT id FROM users WHERE login = 'admin' LIMIT 1), 0), 1, datetime('now'), datetime('now')
		WHERE NOT EXISTS (SELECT 1 FROM vehicle_templates WHERE name = '%s')
	`, name, name)))

	var parts []string
	for _, fn := range pikoXpEn57Functions {
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

func seedPikoXpEn57TemplateDown(s *rel.Schema) {
	name := sqlLiteral(pikoXpEn57TemplateName)
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM dcc_functions
		WHERE template_id IN (SELECT id FROM vehicle_templates WHERE name = '%s')
	`, name)))
	s.Exec(rel.Raw(fmt.Sprintf(`
		DELETE FROM vehicle_templates WHERE name = '%s'
	`, name)))
}
