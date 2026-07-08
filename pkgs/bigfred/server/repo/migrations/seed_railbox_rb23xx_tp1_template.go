package migrations

import (
	"github.com/go-rel/rel"
)

const railboxRb23xxTp1TemplateName = "RB23xx / Tp1"


// railboxRb23xxTp1Functions is the F0–F28 mapping from the RailBOX
// RB23xx sound pack for Tp1 steam locomotives.
var railboxRb23xxTp1Functions = []templateFunctionSeed{
	{0, "F0", "light"},
	{1, "Silnik", "engine"},
	{2, "Gwizdek długi", "horn_high"},
	{3, "Gwizdek krótki", "horn_low"},
	{5, "Światła długie", "headlight"},
	{6, "Tryb manewrowy", "shunting_mode"},
	{9, "Gwizdek", "whistle"},
	{10, "Sprzęganie", "coupling"},
	{11, "Rozsprzęganie", "uncoupling"},
	{12, "Stukot kół", "wheels"},
	{13, "Skrzypienie kół", "wheel_squeal"},
	{14, "Hamulec", "brake_sound"},
	{15, "Zapowiedź 1", "speaker"},
	{16, "Zapowiedź 2", "speaker"},
	{17, "Dzwon", "bell"},
	{18, "Para", "steam_release"},
	{19, "Węgel", "coal_bunker"},
	{20, "Nawadnianie", "watering"},
	{21, "Piaskowanie", "sander"},
	{22, "Wył. dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie", "mute_sounds"},
	{24, "Nawęglanie", "coal_shoveling"},
	{28, "Wi-Fi", "wifi"},
}

func seedRailboxRb23xxTp1TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, railboxRb23xxTp1TemplateName, railboxRb23xxTp1Functions)
}

func seedRailboxRb23xxTp1TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, railboxRb23xxTp1TemplateName)
}
