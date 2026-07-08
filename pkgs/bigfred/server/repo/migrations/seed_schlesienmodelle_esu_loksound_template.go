package migrations

import (
	"github.com/go-rel/rel"
)

const schlesienModelleEsuLoksoundTemplateName = "SchlesienModelle / ESU Loksound"


// schlesienModelleEsuLoksoundFunctions is the F0–F27 mapping from the
// SchlesienModelle ESU Loksound decoder leaflet.
var schlesienModelleEsuLoksoundFunctions = []templateFunctionSeed{
	{0, "Krótkie światła zależne od kierunku jazdy", "light"},
	{1, "Dźwięk", "sound"},
	{2, "Wysoki ton syreny – włącz/wyłącz", "horn_high"},
	{3, "Sygnał Pc5", "side_lights"},
	{4, "Długie światła zależne od kierunku jazdy (musi być włączone F0)", "headlight"},
	{5, "Niski ton syreny – włącz/wyłącz", "horn_low"},
	{6, "Sygnał Tb1", "beacon_light"},
	{7, "Oświetlenie kabiny maszynisty zależne od kierunku jazdy", "cab_light"},
	{8, "Oświetlenie przedziału maszynowego", "engine_room_light"},
	{9, "Sygnał Pc2", "pc2_signal"},
	{10, "Wyłączenie baterii (jeśli F1 = wyłączony)", "mute_sounds"},
	{11, "Pisk zwrotnic", "wheels"},
	{12, "Kompresor", "compressor"},
	{13, "Sygnał Pc6", "shunting_steps_light"},
	{14, "Pisk obrzeży kół", "wheel_squeal"},
	{15, "Kompresor pomocniczy", "compressor"},
	{16, "Sprzęganie/Rozprzęganie", "coupling"},
	{17, "Piaskowanie", "sander"},
	{18, "Klimatyzacja", "fan"},
	{19, "Gwizdek konduktora", "whistle"},
	{20, "Regulacja głośności", "speaker"},
	{21, "Ręczny hamulec – założenie/zwolnienie", "hand_brake"},
	{22, "Otwieranie/zamykanie drzwi", "door"},
	{23, "Otwieranie/zamykanie okna", "window"},
	{24, "Wysoki ton syreny – krótki", "horn_high"},
	{25, "Niski ton syreny – krótki", "horn_low"},
	{26, "Sprzęganie", "coupling"},
	{27, "Rozprzęganie", "uncoupling"},
}

func seedSchlesienModelleEsuLoksoundTemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, schlesienModelleEsuLoksoundTemplateName, schlesienModelleEsuLoksoundFunctions)
}

func seedSchlesienModelleEsuLoksoundTemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, schlesienModelleEsuLoksoundTemplateName)
}
