package migrations

import (
	"github.com/go-rel/rel"
)

const rocoZimo810TemplateName = "Roco / ZIMO / 810"


// rocoZimo810Functions is the F-key mapping from the ZIMO sound project for
// the Roco HO ČD series 810 diesel railcar (MX64x/MX69x, sound version Roco).
// F21–F23 and F25 are unused on this decoder.
var rocoZimo810Functions = []templateFunctionSeed{
	{0, "Światła czołowe zależne od kierunku jazdy", "headlight"},
	{1, "Czerwone światła tylne zależne od kierunku jazdy", "red_lights"},
	{2, "Światła długie zależne od kierunku jazdy", "light"},
	{3, "Tryb manewrowy", "shunting_mode"},
	{4, "Wyłączenie krzywej przyspieszania", "valve"},
	{5, "Oświetlenie wnętrza kabiny maszynisty zależne od kierunku jazdy", "cab_light"},
	{6, "Jazda bez obciążenia", "wheels"},
	{7, "Sygnał trąbkowy 1", "horn_high"},
	{8, "Dźwięk włącz / wyłącz", "sound"},
	{9, "Sygnał trąbkowy 1 – krótki", "horn_low"},
	{10, "Sygnał trąbkowy 2", "horn_high"},
	{11, "Gwizdek 1", "whistle"},
	{12, "Zapowiedź stacyjna", "speaker"},
	{13, "Ogrzewanie", "engine"},
	{14, "Otwieranie / zamykanie drzwi", "door"},
	{15, "Kompresor (dźwięk losowy)", "compressor"},
	{16, "Gwizdek konduktora", "whistle"},
	{17, "Sprzęganie", "coupling"},
	{18, "Rozsprzęganie", "uncoupling"},
	{19, "Zestaw dźwięków – z ładunkiem / bez ładunku", "sound"},
	{20, "Skrzypienie kół na łukach", "wheel_squeal"},
	{24, "Piasek", "sander"},
	{26, "Ściszenie dźwięku", "volume_down"},
	{27, "Zwiększenie głośności", "volume_up"},
	{28, "Wyciszenie", "mute_sounds"},
}

func seedRocoZimo810TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, rocoZimo810TemplateName, rocoZimo810Functions)
}

func seedRocoZimo810TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, rocoZimo810TemplateName)
}
