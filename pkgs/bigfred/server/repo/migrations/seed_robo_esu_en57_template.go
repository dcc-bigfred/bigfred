package migrations

import (
	"github.com/go-rel/rel"
)

const roboEsuEn57TemplateName = "ROBO / ESU Loksound / EN57"


// roboEsuEn57Functions is the F0–F28 mapping for the ROBO ESU Loksound EN57.
var roboEsuEn57Functions = []templateFunctionSeed{
	{0, "Włączanie / wyłączanie świateł białych przednich i czerwonych tylnych zmiennych kierunkowo", "headlight"},
	{1, "Włączanie / wyłączanie dźwięku", "sound"},
	{2, "Trąbka wysoki ton", "horn_high"},
	{3, "Oświetlenie przedziału pasażerskiego", "interior_light"},
	{4, "Oświetlenie tablicy relacyjnej", "destination_board_lights"},
	{5, "Oświetlenie czoła pociągu do jazdy po torze w kierunku przeciwnym do zasadniczego (sygnał Pc2)", "pc2_signal"},
	{6, "Oświetlenie manewrowe (zredukowana prędkość oraz czasy rozpędzania i hamowania)", "shunting_mode"},
	{7, "Przyciemnianie świateł białych", "light"},
	{8, "Wyłączanie oświetlenia kabiny maszynisty", "cab_light"},
	{9, "Otwieranie / zamykanie drzwi wejściowych", "door"},
	{10, "Sprężarka", "compressor"},
	{11, "Podnoszenie / opuszczanie pantografu", "pantograph"},
	{12, "Trąbka dwutonowa", "horn_high"},
	{13, "Zapowiedź stacyjna #1", "speaker"},
	{14, "Zapowiedź stacyjna #2", "speaker"},
	{15, "Rozmowa przez radiotelefon #1", "radio_command"},
	{16, "Rozmowa przez radiotelefon #2", "radio_command"},
	{17, "Trąbka niski ton", "horn_low"},
	{18, "Włączenie / luzowanie hamulca", "brake_sound"},
	{19, "Wyciszenie dźwięku", "mute_sounds"},
	{20, "Gwizdek konduktora", "whistle"},
	{21, "Wyłączenie dźwięków hamowania", "brake_sound_mute"},
	{22, "Jazda po rozjazdach", "wheels"},
	{23, "Stukot kół na łączeniach szyn", "wheels"},
	{24, "Wentylator", "fan"},
	{25, "Prędkościomierz (Hasler)", "dashboard_light"},
	{26, "Wydmuch sprężonego powietrza", "steam_release"},
	{27, "Sprzęganie", "coupling"},
	{28, "Rozmowa przez radiotelefon #3", "radio_command"},
}

func seedRoboEsuEn57TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, roboEsuEn57TemplateName, roboEsuEn57Functions)
}

func seedRoboEsuEn57TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, roboEsuEn57TemplateName)
}
