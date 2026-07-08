package migrations

import (
	"github.com/go-rel/rel"
)

const rocoDcc24EsuTy2TemplateName = "Roco / DCC24 / ESU Loksound / Ty2"


// rocoDcc24EsuTy2Functions is the F0–F31 mapping for the Roco DCC24 ESU Loksound Ty2.
var rocoDcc24EsuTy2Functions = []templateFunctionSeed{
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
	seedTemplateFunctions(s, rocoDcc24EsuTy2TemplateName, rocoDcc24EsuTy2Functions)
}

func seedRocoDcc24EsuTy2TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, rocoDcc24EsuTy2TemplateName)
}
