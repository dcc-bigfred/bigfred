package migrations

import (
	"github.com/go-rel/rel"
)

const pikoDcc24EsuSp45Su45TemplateName = "PIKO / DCC24 / ESU Loksound / SP45 - SU45"


// pikoDcc24EsuSp45Su45Functions is the F0–F24 mapping from the PIKO DCC24
// ESU Loksound decoder leaflet for SP45/SU45.
var pikoDcc24EsuSp45Su45Functions = []templateFunctionSeed{
	{0, "Włączenie / wyłączenie świateł białych zmiennych kierunkowo", "headlight"},
	{1, "Uruchomienie / wyłączenie lokomotywy oraz dźwięków jazdy", "engine"},
	{2, "Trąbka krótka", "horn_low"},
	{3, "Jazda manewrowa (zredukowana prędkość oraz czasy rozpędzania i hamowania) + światła manewrowe + dźwięk rozprzęgania", "shunting_mode"},
	{4, "Światła czerwone zmienne kierunkowo", "red_lights"},
	{5, "Oświetlenie kabiny zmienne kierunkowo", "cab_light"},
	{6, "Światła długie", "light"},
	{7, "Dźwięk wentylatorów", "fan"},
	{8, "Trąbka długa", "horn_high"},
	{9, "Gwizdek konduktora", "whistle"},
	{10, "Zapowiedź stacyjna #1", "speaker"},
	{11, "Załączone ogrzewanie elektryczne – podniesienie obrotów na postoju", "engine"},
	{12, "Trąbka – inny ton", "bell"},
	{13, "Tarcie kół o szyny na łukach i rozjazdach", "wheel_squeal"},
	{14, "Zapowiedź stacyjna #2", "speaker"},
	{15, "Wydmuch powietrza", "steam_release"},
	{16, "Otwieranie / zamykanie drzwi", "door"},
	{17, "Włączenie / luzowanie hamulca", "brake_sound"},
	{18, "Światła postojowe", "side_lights"},
	{19, "Piasecznica", "sander"},
	{20, "Stukot kół na łączeniach szyn", "wheels"},
	{21, "Wyciszenie dźwięków do 50%", "mute_sounds"},
	{22, "Deaktywacja dźwięku hamowania", "brake_sound_mute"},
	{23, "Deaktywacja turbosprężarki", "compressor"},
	{24, "Klakson", "bell"},
}

func seedPikoDcc24EsuSp45Su45TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, pikoDcc24EsuSp45Su45TemplateName, pikoDcc24EsuSp45Su45Functions)
}

func seedPikoDcc24EsuSp45Su45TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, pikoDcc24EsuSp45Su45TemplateName)
}
