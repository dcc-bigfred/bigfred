package migrations

import (
	"github.com/go-rel/rel"
)

const railboxRb23xxSt44TemplateName = "RB23xx / ST44"


// railboxRb23xxSt44Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for ST44 locomotives.
var railboxRb23xxSt44Functions = []templateFunctionSeed{
	{0, "Światła pociągowe", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka Wysoka", "horn_high"},
	{3, "Trąbka Niska", "horn_low"},
	{5, "Światło wewnętrzne", "interior_light"},
	{6, "Tryb manewrowy", "shunting_mode"},
	{7, "Światła tylne", "red_lights"},
	{8, "Światła kabiny", "cab_light"},
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

func seedRailboxRb23xxSt44TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, railboxRb23xxSt44TemplateName, railboxRb23xxSt44Functions)
}

func seedRailboxRb23xxSt44TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, railboxRb23xxSt44TemplateName)
}
