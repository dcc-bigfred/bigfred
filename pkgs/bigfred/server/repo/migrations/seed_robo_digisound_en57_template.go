package migrations

import (
	"github.com/go-rel/rel"
)

const roboDigisoundEn57TemplateName = "ROBO / Digisound / EN57"


// roboDigisoundEn57Functions is the F0–F26 mapping for the ROBO Digisound EN57.
var roboDigisoundEn57Functions = []templateFunctionSeed{
	{0, "Włączanie / wyłączanie świateł białych przednich i czerwonych tylnych zmiennych kierunkowo", "headlight"},
	{1, "Włączanie / wyłączanie dźwięku", "sound"},
	{2, "Trąbka wysoki ton", "horn_high"},
	{3, "Oświetlenie przedziału pasażerskiego", "interior_light"},
	{4, "Oświetlenie tablicy relacyjnej", "destination_board_lights"},
	{5, "Oświetlenie czoła pociągu do jazdy po torze w kierunku przeciwnym do zasadniczego (sygnał Pc2)", "pc2_signal"},
	{6, "Oświetlenie manewrowe (zredukowana prędkość oraz czasy rozpędzania i hamowania)", "shunting_mode"},
	{7, "Przyciemnianie świateł białych", "light"},
	{8, "Oświetlenie kabiny maszynisty", "cab_light"},
	{9, "Otwieranie / zamykanie drzwi wejściowych", "door"},
	{10, "Sprężarka", "compressor"},
	{11, "Trąbka krótki dźwięk", "horn_low"},
	{12, "Zapowiedź stacyjna #1", "speaker"},
	{13, "Zapowiedź stacyjna #2", "speaker"},
	{14, "Rozmowa przez radiotelefon #1", "radio_command"},
	{15, "Rozmowa przez radiotelefon #2", "radio_command"},
	{16, "Trąbka niski ton", "horn_low"},
	{17, "Hamowanie / luzowanie hamulca", "brake_sound"},
	{18, "Gwizdek konduktora", "whistle"},
	{19, "Otwieranie / zamykanie drzwi w przedziale pasażerskim", "door"},
	{20, "Dźwięk Haslera (prędkościomierza) i nastawnika w kabinie maszynisty (działa przy włączonym F1)", "dashboard_light"},
	{21, "Wycieraczki", "wipers"},
	{22, "Zgrzyt kół na rozjazdach", "wheel_squeal"},
	{23, "Stukot kół na łączeniach szyn", "wheels"},
	{24, "Hamulec ręczny", "hand_brake"},
	{25, "Wyciszenie dźwięku", "mute_sounds"},
	{26, "Zmiana głośności dźwięków", "speaker"},
}

func seedRoboDigisoundEn57TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, roboDigisoundEn57TemplateName, roboDigisoundEn57Functions)
}

func seedRoboDigisoundEn57TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, roboDigisoundEn57TemplateName)
}
