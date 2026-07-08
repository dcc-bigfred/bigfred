package migrations

import (
	"github.com/go-rel/rel"
)

const railboxRb23xxSm31TemplateName = "RB23xx / SM31"


// railboxRb23xxSm31Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for SM31 locomotives.
var railboxRb23xxSm31Functions = []templateFunctionSeed{
	{0, "Światła pociągowe", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka Wysoka", "horn_high"},
	{3, "Trąbka Niska", "horn_low"},
	{5, "Światła długie", "headlight"},
	{6, "Tryb manewrowy", "shunting_mode"},
	{9, "Gwizdek", "whistle"},
	{10, "Sprzęganie", "coupling"},
	{11, "Rozsprzęganie", "uncoupling"},
	{12, "Stukot kół", "wheels"},
	{13, "Skrzypienie kół", "wheel_squeal"},
	{14, "Hamulec", "brake_sound"},
	{17, "Sprężarka", "compressor"},
	{18, "Ciśnienie", "steam_release"},
	{19, "Pompa oleju", "oil_pump"},
	{20, "Drzwi lokomotywy", "door"},
	{21, "Piaskowanie", "sander"},
	{22, "Wył. dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie", "mute_sounds"},
	{27, "Sygnał Pc2", "pc2_signal"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxSm31TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, railboxRb23xxSm31TemplateName, railboxRb23xxSm31Functions)
}

func seedRailboxRb23xxSm31TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, railboxRb23xxSm31TemplateName)
}
