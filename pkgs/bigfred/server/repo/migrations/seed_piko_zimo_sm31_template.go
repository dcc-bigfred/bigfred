package migrations

import (
	"github.com/go-rel/rel"
)

const pikoZimoSm31TemplateName = "PIKO / DCC24 / ZIMO / SM31"


// pikoZimoSm31Functions is the F0–F28 mapping from the PIKO DCC24 ZIMO
// decoder leaflet for the SM31 locomotive.
var pikoZimoSm31Functions = []templateFunctionSeed{
	{0, "Światła białe + podświetlenie pulpitu maszynisty zmienne kierunkowo + dźwięk przełącznika", "headlight"},
	{1, "Włączenie / wyłączenie dźwięku silnika spalinowego a8C22W", "engine"},
	{2, "Trąbka długa", "horn_high"},
	{3, "Trąbka krótka", "horn_low"},
	{4, "Światła czerwone zmienne kierunkowo + dźwięk przełącznika", "red_lights"},
	{5, "Oświetlenie kabiny maszynisty + dźwięk przełącznika", "cab_light"},
	{6, "Tryb jazdy manewrowej (zredukowane czasy zwalniania i przyspieszania, zredukowana prędkość) + światła manewrowe zmienne + dźwięk sprzęgania / rozsprzęgania", "shunting_mode"},
	{7, "Tarcie kół o szyny na łukach", "wheel_squeal"},
	{8, "Światła do jazdy po torze przeciwnym do zasadniczego (sygnał Pc2)", "pc2_signal"},
	{9, "Wydmuch sprężonego powietrza", "steam_release"},
	{10, "Gwizdek konduktora", "whistle"},
	{11, "Wentylator", "fan"},
	{12, "Światła mocne / słabe", "light"},
	{13, "Sprężarka (włącza się też automatycznie w losowych odstępach czasu)", "compressor"},
	{14, "Zapowiedź stacyjna", "speaker"},
	{15, "Oświetlenie rewizyjne podwozia", "undercarriage_light"},
	{16, "Otwieranie / zamykanie drzwi kabiny", "door"},
	{17, "Ręczne podniesienie obrotów silnika do maksimum", "volume_up"},
	{18, "Klakson", "bell"},
	{19, "Pompa paliwa", "oil_pump"},
	{20, "Podgrzewacz (Webasto)", "engine"},
	{21, "Radiotelefon #1", "radio_command"},
	{22, "Radiotelefon #2", "radio_command"},
	{23, "Czuwak aktywny", "sifa"},
	{24, "Tachograf (Hasler)", "dashboard_light"},
	{25, "Piasecznice", "sander"},
	{26, "Wyciszenie dźwięku (tryb tunelu)", "mute_sounds"},
	{27, "Zmniejszenie głośności (Vol-)", "volume_down"},
	{28, "Zwiększenie głośności (Vol+)", "volume_up"},
}

func seedPikoZimoSm31TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, pikoZimoSm31TemplateName, pikoZimoSm31Functions)
}

func seedPikoZimoSm31TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, pikoZimoSm31TemplateName)
}
