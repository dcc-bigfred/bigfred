package migrations

import (
	"github.com/go-rel/rel"
)

const rocoDcc24ZimoTy2TemplateName = "Roco / DCC24 / ZIMO / Ty2"


// rocoDcc24ZimoTy2Functions is the F0–F23 mapping for the Roco DCC24 ZIMO Ty2.
var rocoDcc24ZimoTy2Functions = []templateFunctionSeed{
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
	seedTemplateFunctions(s, rocoDcc24ZimoTy2TemplateName, rocoDcc24ZimoTy2Functions)
}

func seedRocoDcc24ZimoTy2TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, rocoDcc24ZimoTy2TemplateName)
}
