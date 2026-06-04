### 3a.8 Function icon catalogue

`FunctionIcon` is a **closed** enumeration. The backend accepts only values
from this list (`PUT …/functions/{num}` returns `422` for unknown icons).
The frontend ships one SVG asset per value; labels are translated via
`function.icon.<slug>` keys (see [§7c i18n](../09a-i18n.md)).

The table below is authoritative. The **slug** column is the wire value and
the Go / TypeScript enum member; the **label (PL)** column is the default
Polish caption shown in the icon picker and in tooltips.

| Slug | Label (PL) |
|------|------------|
| `unspecified` | Funkcja nieokreślona |
| `light` | Światło |
| `engine` | Silnik |
| `sound` | Dźwięk |
| `horn` | Klakson |
| `coupler` | Sprzęg |
| `interior_light` | Światło w środku |
| `engine_room_light` | Światło przedziału maszynowego |
| `shunting_steps_light` | Światło stopni manewrowych |
| `inspection_light` | Światło rewizyjne |
| `cab_light` | Światło kabiny |
| `headlight` | Światła czołowe |
| `roof_headlight` | Reflektor górny |
| `red_lights` | Światła czerwone |
| `vestibule_lights` | Światła przedsionka |
| `destination_board_lights` | Światła tablicy |
| `door` | Drzwi |
| `smoke` | Dym |
| `speaker` | Głośnik |
| `whistle` | Gwizdek |
| `toilet` | WC |
| `compressor` | Kompresor |
| `brake_sound` | Dźwięk hamulca on/off |
| `coal_shoveling` | Szuflowanie węgla |
| `fan` | Wentylator |
| `hand_brake` | Hamulec ręczny |
| `injector` | Inżektor |
| `mute_sounds` | Wycisz dźwięki |
| `radio_command` | Polecenie radiowe |
| `shunting_mode` | Tryb manewrowy |
| `valve` | Zawór |
| `wheels` | Koła |
| `wipers` | Wycieraczki |
| `sander` | Piasek |
| `long_whistle` | Długi gwizdek |
| `short_whistle` | Krótki gwizdek |
| `pantograph` | Pantograf |
| `volume_up` | Zwiększ głośność |
| `volume_down` | Zmniejsz głośność |
| `heavy_load` | Ciężki ładunek |
| `wifi` | Wi-Fi |
| `pc2_signal` | Sygnał Pc2 |
| `coupling` | Sprzęganie |
| `uncoupling` | Rozsprzęganie |
| `oil_pump` | Pompa oleju |
| `brake_sound_mute` | Wyłączenie dźwięków hamowania |
| `wheel_squeal` | Skrzypienie kół |
| `bell` | Dzwon |
| `coal_bunker` | Nawęgalnie |
| `watering` | Nawdnianie |
| `crane_up` | Dźwig w górę |
| `crane_down` | Dźwig w dół |
| `crane_left` | Dźwig w lewo |
| `crane_right` | Dźwig w prawo |
| `crane_hook` | Hak dźwigu |
| `sifa` | SIFA |
| `firebox` | Palenisko |
| `steam_release` | Wypuszczanie pary |
| `window` | Okno |
| `buffer` | Bufor |
| `danger` | Niebezpieczeństwo |
| `engineer_laugh` | Śmiech maszynisty |
| `stairs` | Schody |
| `beacon_light` | Światło obrotowe |
| `side_lights` | Światła boczne |
| `turn_signal_left` | Kierunkowskaz lewy |
| `turn_signal_right` | Kierunkowskaz prawy |

`GET /api/v1/function-icons` returns this list in a stable order (the table
order above). Scripts reuse the same catalogue for their button icons (§3a.7).

**Legacy aliases** — older drafts used shorter names (`high_horn`,
`low_horn`, `headlight`, …). Implementations MUST map those to the slugs
above or reject them; the catalogue in this section supersedes the partial
list in [§3a.1](./01-entities.md).
