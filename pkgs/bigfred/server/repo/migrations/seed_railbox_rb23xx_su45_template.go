package migrations

import (
	"github.com/go-rel/rel"
)

const railboxRb23xxSu45TemplateName = "RB23xx / SU45"


// railboxRb23xxSu45Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for SU45 locomotives.
var railboxRb23xxSu45Functions = []templateFunctionSeed{
	{0, "F0", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka Wysoka", "horn_high"},
	{3, "Trąbka Niska", "horn_low"},
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
	{19, "Pompa oleju", "oil_pump"},
	{20, "Drzwi lokomotywy", "door"},
	{21, "Piaskowanie", "sander"},
	{22, "Prądnica ogrzewcza", "engine"},
	{23, "Wyciszenie", "mute_sounds"},
	{25, "Drzwi wagonu 1", "door"},
	{26, "Drzwi wagonu", "door"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxSu45TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, railboxRb23xxSu45TemplateName, railboxRb23xxSu45Functions)
}

func seedRailboxRb23xxSu45TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, railboxRb23xxSu45TemplateName)
}
