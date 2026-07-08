package migrations

import (
	"github.com/go-rel/rel"
)

const pikoXpSp45Su45TemplateName = "PIKO / XP / SP45 - SU45"


// pikoXpSp45Su45Functions is the F0–F22 mapping from the PIKO SmartDecoder
// XP Sound leaflet for SP/SU 45 (PKP), product #56561.
var pikoXpSp45Su45Functions = []templateFunctionSeed{
	{0, "Światła", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka wysoki ton", "horn_high"},
	{3, "Trąbka niski ton", "horn_low"},
	{4, "Pompa paliwa", "oil_pump"},
	{5, "Zestaw manewrowy", "shunting_mode"},
	{6, "Podgrzewacz", "engine"},
	{7, "Zapowiedź stacyjna", "speaker"},
	{8, "Gwizdek konduktora", "whistle"},
	{9, "Oświetlenie kabiny 1", "cab_light"},
	{10, "Oświetlenie kabiny 2", "cab_light"},
	{11, "Radio", "radio_command"},
	{12, "Sprężone powietrze", "steam_release"},
	{13, "Sprzęganie", "coupling"},
	{14, "Sprężarka", "compressor"},
	{15, "Hamulec ręczny", "hand_brake"},
	{16, "Stukot kół na łączeniach szyn", "wheels"},
	{17, "Skrzypienie kół na łukach", "wheel_squeal"},
	{18, "Wyłączenie dźwięku hamowania", "brake_sound_mute"},
	{19, "Oświetlenie pociągu: lok pchający", "red_lights"},
	{20, "Oświetlenie pociągu: lok ciągnący", "headlight"},
	{21, "Regulacja głośności", "speaker"},
	{22, "Tryb tunelu", "mute_sounds"},
}

func seedPikoXpSp45Su45TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, pikoXpSp45Su45TemplateName, pikoXpSp45Su45Functions)
}

func seedPikoXpSp45Su45TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, pikoXpSp45Su45TemplateName)
}
