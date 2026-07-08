package migrations

import (
	"github.com/go-rel/rel"
)

const pikoSm31TemplateName = "PIKO / DCC24 / ESU / SM31"


// pikoSm31Functions is the F0–F31 mapping from the PIKO DCC24 ESU decoder
// leaflet for the SM31 locomotive.
var pikoSm31Functions = []templateFunctionSeed{
	{0, "Światła białe + podświetlenie pulpitu maszynisty zmienne kierunkowo + dźwięk przełącznika", "headlight"},
	{1, "Włączenie / wyłączenie dźwięku silnika spalinowego a8C22W", "engine"},
	{2, "Trąbka długa", "horn_high"},
	{3, "Trąbka krótka", "horn_low"},
	{4, "Światła czerwone zmienne kierunkowo + dźwięk przełącznika", "red_lights"},
	{5, "Oświetlenie kabiny maszynisty + dźwięk przełącznika", "cab_light"},
	{6, "Tryb jazdy manewrowej (zredukowane czasy zwalniania i przyspieszania, zredukowana prędkość) + światła manewrowe zmienne (przy wyłączonym F0 stałe tylko po prawej stronie)", "shunting_mode"},
	{7, "Tarcie kół o szyny na łukach", "wheel_squeal"},
	{8, "Światła do jazdy po torze przeciwnym do zasadniczego (sygnał Pc2)", "pc2_signal"},
	{9, "Wydmuch sprężonego powietrza", "steam_release"},
	{10, "Gwizdek konduktora", "whistle"},
	{11, "Dźwięk sprzęgania / rozsprzęgania", "coupling"},
	{12, "Światła mocne / słabe", "light"},
	{13, "Uruchomienie / zwolnienie hamulca lokomotywy", "brake_sound"},
	{14, "Zapowiedź stacyjna", "speaker"},
	{15, "Oświetlenie rewizyjne podwozia", "undercarriage_light"},
	{16, "Otwieranie / zamykanie drzwi kabiny", "door"},
	{17, "Przycisk automatycznego hamowania (po włączeniu tej funkcji model się zatrzymuje, po wyłączeniu rusza i rozpędza się do pierwotnej prędkości)", "brake_sound"},
	{18, "Klakson", "bell"},
	{19, "Hamulec ręczny", "hand_brake"},
	{20, "Stukot kół", "wheels"},
	{21, "Odgłos przejazdu przez rozjazdy", "wheels"},
	{22, "Radiotelefon #1", "radio_command"},
	{23, "Wentylator", "fan"},
	{24, "Radiotelefon #2", "radio_command"},
	{25, "Piasecznice", "sander"},
	{26, "Wyciszenie dźwięku o 50% (tryb tunelu)", "mute_sounds"},
	{27, "Wycieraczka okienna (2 tryby pracy ustawiane przez CV168, 0 lub 1)", "wipers"},
	{28, "Hamulec pociągowy", "brake_sound"},
	{29, "Pompa paliwa", "oil_pump"},
	{30, "Sprężarka (włącza się też automatycznie w losowych odstępach czasu)", "compressor"},
	{31, "Ręczne podniesienie obrotów silnika do maksimum", "volume_up"},
}

// seedPikoSm31TemplateUp inserts the PIKO / DCC24 / ESU / SM31 catalogue
// template with all 32 function slots. The owner is the bootstrap admin
// when that account already exists; otherwise owner_user_id stays 0 until
// an admin is seeded (only admins can edit owner-less catalogue rows).
func seedPikoSm31TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, pikoSm31TemplateName, pikoSm31Functions)
}

func seedPikoSm31TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, pikoSm31TemplateName)
}
