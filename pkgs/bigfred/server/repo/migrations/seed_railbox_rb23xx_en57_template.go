package migrations

import (
	"github.com/go-rel/rel"
)

const railboxRb23xxEn57TemplateName = "RB23xx / EN57"


// railboxRb23xxEn57Functions is the F0–F31 mapping from the RailBOX
// RB23xx sound pack for EN57 EMUs.
var railboxRb23xxEn57Functions = []templateFunctionSeed{
	{0, "F0", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka Wysoka", "horn_high"},
	{3, "Trąbka Niska", "horn_low"},
	{4, "Wentylator", "fan"},
	{5, "Światło wewnętrzne", "interior_light"},
	{6, "Tryb manewrowy", "shunting_mode"},
	{7, "Światła tylne", "red_lights"},
	{8, "Światła kabiny", "cab_light"},
	{9, "Gwizdek", "whistle"},
	{10, "Sprzęganie", "coupling"},
	{11, "Rozsprzęganie", "uncoupling"},
	{12, "Stukot kół", "wheels"},
	{13, "Skrzypienie kół", "wheel_squeal"},
	{14, "Hałas od gum", "buffer"},
	{15, "Zapowiedź 1", "speaker"},
	{16, "Zapowiedź 2", "speaker"},
	{17, "Sprężarka", "compressor"},
	{18, "Ciśnienie", "steam_release"},
	{19, "Sprężarka 2", "compressor"},
	{20, "Info ->", "turn_signal_right"},
	{21, "Info <-", "turn_signal_left"},
	{22, "Wył. dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie", "mute_sounds"},
	{24, "Pantograf", "pantograph"},
	{25, "Drzwi wagonu", "door"},
	{26, "Bzyczek", "bell"},
	{27, "Sygnał Pc2", "pc2_signal"},
	{28, "Wi-Fi", "wifi"},
	{29, "Drzwi kabiny", "door"},
	{30, "Piaskowanie", "sander"},
	{31, "Światła tablicy", "destination_board_lights"},
}

func seedRailboxRb23xxEn57TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, railboxRb23xxEn57TemplateName, railboxRb23xxEn57Functions)
}

func seedRailboxRb23xxEn57TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, railboxRb23xxEn57TemplateName)
}
