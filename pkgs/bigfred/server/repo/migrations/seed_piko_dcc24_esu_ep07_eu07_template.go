package migrations

import (
	"github.com/go-rel/rel"
)

const pikoDcc24EsuEp07Eu07TemplateName = "PIKO / DCC24 / ESU LOKSOUND / EP07 - EU07"


// pikoDcc24EsuEp07Eu07Functions is the F0–F24 mapping from the PIKO DCC24
// ESU Loksound decoder leaflet for EP07/EU07.
var pikoDcc24EsuEp07Eu07Functions = []templateFunctionSeed{
	{0, "Włączenie / wyłączenie świateł białych zmiennych kierunkowo", "headlight"},
	{1, "Uruchomienie / wyłączenie lokomotywy oraz dźwięków jazdy", "engine"},
	{2, "Trąbka wysokotonowa", "horn_high"},
	{3, "Jazda manewrowa (zredukowana prędkość oraz czasy rozpędzania i hamowania) + światła manewrowe + dźwięk rozprzęgania", "shunting_mode"},
	{4, "Światła czerwone zmienne kierunkowo", "red_lights"},
	{5, "Oświetlenie kabiny zmienne kierunkowo", "cab_light"},
	{6, "Światła długie", "light"},
	{7, "Kompresor", "compressor"},
	{8, "Trąbka niskotonowa", "horn_low"},
	{9, "Gwizdek konduktora", "whistle"},
	{10, "Zapowiedź stacyjna #1", "speaker"},
	{11, "Wyłączenie dźwięku wentylatorów chłodzenia oporników rozruchowych", "fan"},
	{12, "Uszkodzona trąbka", "bell"},
	{13, "Tarcie kół o szyny na łukach i rozjazdach", "wheel_squeal"},
	{14, "Kompresor pomocniczy", "compressor"},
	{15, "Zapowiedź stacyjna #2", "speaker"},
	{16, "Otwieranie / zamykanie drzwi", "door"},
	{17, "Włączenie / zwolnienie hamulca", "brake_sound"},
	{18, "Światła postojowe", "side_lights"},
	{19, "Piasecznica", "sander"},
	{20, "Stukot kół na łączeniach szyn", "wheels"},
	{21, "Wydmuch sprężonego powietrza", "steam_release"},
	{22, "Deaktywacja dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie dźwięku", "mute_sounds"},
	{24, "Sygnał dźwiękowy alarmowy", "danger"},
}

func seedPikoDcc24EsuEp07Eu07TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, pikoDcc24EsuEp07Eu07TemplateName, pikoDcc24EsuEp07Eu07Functions)
}

func seedPikoDcc24EsuEp07Eu07TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, pikoDcc24EsuEp07Eu07TemplateName)
}
