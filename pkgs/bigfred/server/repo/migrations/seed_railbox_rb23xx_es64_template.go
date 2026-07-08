package migrations

import (
	"github.com/go-rel/rel"
)

const railboxRb23xxEs64TemplateName = "RB23xx / ES64"


// railboxRb23xxEs64Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for ES64 locomotives.
var railboxRb23xxEs64Functions = []templateFunctionSeed{
	{0, "Światła pociągowe", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka Wysoka", "horn_high"},
	{3, "Trąbka Niska", "horn_low"},
	{4, "Wentylator", "fan"},
	{5, "Światła długie", "headlight"},
	{6, "Tryb manewrowy", "shunting_mode"},
	{7, "Światła tylne", "red_lights"},
	{8, "Światła kabiny", "cab_light"},
	{9, "Gwizdek", "whistle"},
	{10, "Sprzęganie", "coupling"},
	{11, "Rozsprzęganie", "uncoupling"},
	{12, "Stukot kół", "wheels"},
	{13, "Skrzypienie kół", "wheel_squeal"},
	{14, "Hamulec", "brake_sound"},
	{15, "Zapowiedź 1", "speaker"},
	{17, "Sprężarka", "compressor"},
	{18, "Ciśnienie", "steam_release"},
	{20, "Drzwi lokomotywy", "door"},
	{21, "Piaskowanie", "sander"},
	{22, "Wył. dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie", "mute_sounds"},
	{24, "Pantograf", "pantograph"},
	{25, "Drzwi wagonu 1", "door"},
	{27, "Sygnał Pc2", "pc2_signal"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxEs64TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, railboxRb23xxEs64TemplateName, railboxRb23xxEs64Functions)
}

func seedRailboxRb23xxEs64TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, railboxRb23xxEs64TemplateName)
}
