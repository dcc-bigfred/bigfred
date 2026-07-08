package migrations

import (
	"github.com/go-rel/rel"
)

const railboxRb23xxEt22TemplateName = "RB23xx / ET22"


// railboxRb23xxEt22Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for ET22 locomotives.
var railboxRb23xxEt22Functions = []templateFunctionSeed{
	{0, "Światła pociągowe", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka Wysoka", "horn_high"},
	{3, "Trąbka Niska", "horn_low"},
	{4, "Podświetlenie przedziału maszynowego", "engine_room_light"},
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
	{19, "Mała sprężarka", "compressor"},
	{20, "Drzwi lokomotywy", "door"},
	{21, "Piaskowanie", "sander"},
	{22, "Wył. dźwięk hamowania", "brake_sound_mute"},
	{23, "Wyciszenie", "mute_sounds"},
	{24, "Pantograf", "pantograph"},
	{25, "Drzwi wagonu 1", "door"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxEt22TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, railboxRb23xxEt22TemplateName, railboxRb23xxEt22Functions)
}

func seedRailboxRb23xxEt22TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, railboxRb23xxEt22TemplateName)
}
