package migrations

import (
	"github.com/go-rel/rel"
)

const schlesienModelleDcc24EsuEp07V3TemplateName = "SchlesienModelle / DCC24 / ESU LokSound / EP07 v3"


// schlesienModelleDcc24EsuEp07V3Functions is the F0–F31 mapping from the
// SchlesienModelle DCC24 ESU LokSound decoder leaflet for EP07 v3.
var schlesienModelleDcc24EsuEp07V3Functions = []templateFunctionSeed{
	{0, "Włączenie / wyłączenie świateł białych i czerwonych zmiennych kierunkowo", "light"},
	{1, "Uruchomienie / wyłączenie lokomotywy oraz dźwięków jazdy", "engine"},
	{2, "Trąbka wysokotonowa", "horn_high"},
	{3, "Jazda manewrowa (zredukowana prędkość oraz czasy rozpędzania i hamowania) + światła manewrowe + dźwięk rozprzęgania", "shunting_mode"},
	{4, "Światła Pc2 (jazda po torze lewym w kierunku przeciwnym do zasadniczego)", "pc2_signal"},
	{5, "Oświetlenie kabiny zmienne kierunkowo", "cab_light"},
	{6, "Światła mocne / słabe", "light"},
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
	{18, "Wyłączenie czerwonych świateł (nie działa w pierwszej edycji EP07-361)", "red_lights"},
	{19, "Piasecznica", "sander"},
	{20, "Stukot kół na łączeniach szyn", "wheels"},
	{21, "Wydmuch sprężonego powietrza", "steam_release"},
	{22, "Deaktywacja dźwięku hamowania", "brake_sound_mute"},
	{23, "Wyciszenie dźwięku", "mute_sounds"},
	{24, "Sygnał dźwiękowy alarmowy", "danger"},
	{25, "Wycieraczka okienna", "wipers"},
	{26, "Radio #1", "radio_command"},
	{27, "Radio #2", "radio_command"},
	{28, "Zapowiedź stacyjna", "speaker"},
	{29, "Tachograf - Hasler", "dashboard_light"},
	{30, "Odgłos przejazdu przez rozjazd", "wheels"},
	{31, "Hamulec ręczny", "hand_brake"},
}

func seedSchlesienModelleDcc24EsuEp07V3TemplateUp(s *rel.Schema) {
	seedTemplateFunctions(s, schlesienModelleDcc24EsuEp07V3TemplateName, schlesienModelleDcc24EsuEp07V3Functions)
}

func seedSchlesienModelleDcc24EsuEp07V3TemplateDown(s *rel.Schema) {
	deleteTemplateSeed(s, schlesienModelleDcc24EsuEp07V3TemplateName)
}
