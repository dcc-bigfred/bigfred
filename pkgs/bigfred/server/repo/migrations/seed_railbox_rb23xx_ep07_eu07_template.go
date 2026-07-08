package migrations

import (
	"github.com/go-rel/rel"
)

const railboxRb23xxEp07Eu07TemplateName = "RB23xx / EU07 - EP07"


// railboxRb23xxEp07Eu07Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for EU07 / EP07 locomotives.
var railboxRb23xxEp07Eu07Functions = []templateFunctionSeed{
	{0, "F0", "light"},
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
	{16, "Zapowiedź 2", "speaker"},
	{17, "Sprężarka", "compressor"},
	{18, "Ciśnienie", "steam_release"},
	{19, "Sprężarka mała", "compressor"},
	{20, "Drzwi lokomotywy", "door"},
	{21, "Piaskowanie", "sander"},
	{22, "Wył. dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie", "mute_sounds"},
	{24, "Pantograf", "pantograph"},
	{25, "Drzwi wagonu 1", "door"},
	{26, "Drzwi wagonu 2", "door"},
	{27, "Sygnał Pc2", "pc2_signal"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxEp07Eu07TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, railboxRb23xxEp07Eu07TemplateName, railboxRb23xxEp07Eu07Functions)
}

func seedRailboxRb23xxEp07Eu07TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, railboxRb23xxEp07Eu07TemplateName)
}
