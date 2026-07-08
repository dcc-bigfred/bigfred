package migrations

import (
	"github.com/go-rel/rel"
)

const pikoXpSt44TemplateName = "PIKO / XP / ST44"


// pikoXpSt44Functions is the F0–F28 mapping from the PIKO SmartDecoder XP
// Sound leaflet for ST44 PKP (#56538).
var pikoXpSt44Functions = []templateFunctionSeed{
	{0, "Światła", "light"},
	{1, "Silnik", "engine"},
	{2, "Trąbka", "horn_high"},
	{3, "Pompa paliwa", "oil_pump"},
	{4, "Oświetlenie kabiny", "cab_light"},
	{5, "Pozdrowienie maszynisty", "engineer_laugh"},
	{6, "Oświetlenie przedziału maszynowego", "engine_room_light"},
	{7, "Zestaw przełączników 1", "valve"},
	{8, "Zestaw przełączników 2", "valve"},
	{9, "Wycieraczki", "wipers"},
	{10, "Drzwi kabiny", "door"},
	{11, "Okno kabiny", "window"},
	{12, "Drzwi przedziału maszynowego", "door"},
	{13, "Przełącznik kierunku", "headlight"},
	{14, "Upust zaworu powietrznego", "valve"},
	{15, "Sprzęganie", "coupling"},
	{16, "Hamulec pociągowy", "brake_sound"},
	{17, "Hamulec awaryjny", "danger"},
	{18, "Gwizdek konduktora", "whistle"},
	{19, "Wentylator (kratka)", "fan"},
	{20, "Pedał czuwaka", "sifa"},
	{21, "Hamulec ręczny", "hand_brake"},
	{22, "Piaskowanie", "sander"},
	{23, "Skrzypienie kół na łukach", "wheel_squeal"},
	{24, "Stukot kół na łączeniach szyn", "wheels"},
	{25, "Sygnał Pc1, specjalny sygnał maszynisty", "side_lights"},
	{26, "Sygnał Pc2, jazda po torze przeciwnym", "pc2_signal"},
	{27, "Regulacja głośności", "speaker"},
	{28, "Tryb tunelu", "mute_sounds"},
}

func seedPikoXpSt44TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, pikoXpSt44TemplateName, pikoXpSt44Functions)
}

func seedPikoXpSt44TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, pikoXpSt44TemplateName)
}
