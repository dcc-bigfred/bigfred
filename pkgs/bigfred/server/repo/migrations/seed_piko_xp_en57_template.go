package migrations

import (
	"github.com/go-rel/rel"
)

const pikoXpEn57TemplateName = "PIKO / XP / EN57"


// pikoXpEn57Functions is the F0–F28 mapping from the PIKO SmartDecoder XP
// Sound leaflet for EN57 PKP (#56599).
var pikoXpEn57Functions = []templateFunctionSeed{
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
	seedTemplateFunctions(s, pikoXpEn57TemplateName, pikoXpEn57Functions)
}

func seedPikoXpEn57TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, pikoXpEn57TemplateName)
}
