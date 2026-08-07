---
name: Merge my into available
overview: Usunąć Moje pojazdy/składy; scalić w Dostępne z Odśwież/Dodaj/„Pokaż tylko moje”, filtrami kind/epoch, chipem epoki, podpisanymi akcjami; menu Moje; usunąć GET /vehicles i GET /trains.
todos:
  - id: nav-routes
    content: "Menu Moje: Dostępne pojazdy/składy; usuń Tabor; routing + redirecty fleet→my"
    status: in_progress
  - id: vehicles-toolbar
    content: "AvailableVehicles: Odśwież, Dodaj, Pokaż tylko moje (default on)"
    status: pending
  - id: trains-toolbar
    content: "AvailableTrains: Odśwież, Dodaj skład, Pokaż tylko moje (default on)"
    status: pending
  - id: frontend-my-hooks
    content: Przepiąć TrainDialog/VehicleFunctionsPage z useMy* na catalogue; usuń useMyVehicles/useMyTrains
    status: pending
  - id: backend-cleanup
    content: Usuń GET /vehicles i GET /trains + ListOwned/ListByOwner + testy; zostaw POST/PUT/DELETE i catalogue
    status: pending
  - id: kind-epoch-filters
    content: "Belka pojazdów: filtry po typie (kind) i epoce (+ Pokaż tylko moje)"
    status: pending
  - id: epoch-chip
    content: Chip epoki w szczegółach wiersza pojazdu (jeśli ustawiona, info/niebieski)
    status: pending
  - id: labeled-actions
    content: "Akcje wiersza z podpisami; pojazdy: także Edytuj funkcje (z Moje)"
    status: pending
  - id: help-preserve
    content: Zachować help myVehicles/myTrains na /my/vehicles i /my/trains (FAB z treścią Moje)
    status: pending
  - id: cleanup
    content: Usuń My* pages/catalogues; popraw help/linki/i18n; typecheck + go test
    status: pending
isProject: false
---

# Scalenie Moje → Dostępne (pojazdy i składy)

## Decyzja URL
Kanoniczne ścieżki: **`/my/vehicles`** i **`/my/trains`** (menu Moje). Stare `/fleet/vehicles` i `/fleet/trains` → redirect do `/my/*`. Strona funkcji zostaje pod `/my/vehicles/:vehicleId/functions`.

## Menu ([`AppShell.tsx`](bigfred/web/src/components/AppShell.tsx))
- Usunąć sekcję **Tabor** (`fleetItems` + `TopBarMenu` / mobile section).
- W **Moje** zamiast `nav.my.vehicles` / `nav.my.trains` wstawić pozycje z etykietami **Dostępne pojazdy** / **Dostępne składy** (`nav.fleet.availableVehicles` / `availableTrains`) wskazujące na `/my/vehicles` i `/my/trains`.
- Kolejność w Moje: Dostępne pojazdy, Dostępne składy, potem Wypożyczenia / Reszta jak dziś.

## Usunięcie starych widoków
Skasować (po przeniesieniu brakujących elementów do Available*):
- [`MyVehiclesPage.tsx`](bigfred/web/src/pages/MyVehiclesPage.tsx), [`MyVehiclesCatalogue.tsx`](bigfred/web/src/components/MyVehiclesCatalogue.tsx)
- [`MyTrainsPage.tsx`](bigfred/web/src/pages/MyTrainsPage.tsx), [`MyTrainsCatalogue.tsx`](bigfred/web/src/components/MyTrainsCatalogue.tsx)

W [`App.tsx`](bigfred/web/src/App.tsx):
- Route `/my/vehicles` → `AvailableVehiclesPage`, `/my/trains` → `AvailableTrainsPage`
- Redirect `/fleet/vehicles` → `/my/vehicles`, `/fleet/trains` → `/my/trains`

## Toolbar na Available (pojazdy i składy)

W [`AvailableVehiclesCatalogue.tsx`](bigfred/web/src/components/AvailableVehiclesCatalogue.tsx) i [`AvailableTrainsCatalogue.tsx`](bigfred/web/src/components/AvailableTrainsCatalogue.tsx):

1. **Odśwież listę** — `refetch()` catalogue (+ leases).
2. **Dodaj pojazd** / **Dodaj skład** — otwiera `VehicleDialog` / `TrainDialog` z `null` (create).
3. **Checkbox „Pokaż tylko moje”** — `useState(true)` przy każdym wejściu na widok (bez localStorage); filtr `ownerId === me.id` przed search/paginacją.
4. **Filtr typu pojazdu** — `Select` (single) na belce (opcje z `VEHICLE_KINDS` + „Wszystkie”); filtr `kind`. Single-select (4 zamknięte wartości) — prostsze i wystarczające.
5. **Filtr epoki** — `Select` (single) (opcje z `VEHICLE_EPOCHS` + „Wszystkie” / „Bez epoki”); filtr po `epoch` (puste `epoch` → łapane przez „Bez epoki”).

Kolejność filtrów na belce (pojazdy): checkbox „Pokaż tylko moje” → typ → epoka → Odśwież → Dodaj. Składy: tylko checkbox + Odśwież + Dodaj (bez kind/epoch).

Reset `page` przy zmianie dowolnego filtra (jak przy search). Wartości filtrów w `useState` lokalnym (nie localStorage) — domyślnie typ/epoka = wszystkie, mineOnly = true.

Dla pojazdów: `headerExtra` w [`VehiclesCatalogueTable`](bigfred/web/src/components/vehicles/VehiclesCatalogueTable.tsx) albo toolbar nad tabelą w AvailableVehiclesCatalogue. Filtry kind/epoch mogą być propsami table (`toolbarFilters`) albo żyć w AvailableVehiclesCatalogue i filtrować `rows` przed przekazaniem.

## Epoka w szczegółach wiersza (pojazdy)

W górnym podwierszu atrybutów (obok numeru i chipa „Na makiecie”), gdy `epoch` jest niepuste:

- Chip w stylu jak on-layout, ale **niebieski** — MUI `color="info"` (filled lub outlined).
- Etykieta np. `Epoka {{epoch}}` / `Epoch {{epoch}}` / `Epoche {{epoch}}` (`catalogue.epochChip`), wartość jak w dialogu (`III`, `IVa`, …).
- Puste `epoch` → brak chipa.
- Dodać `epoch` do typu wiersza w `VehiclesCatalogueTable` + mapowanie z catalogue; uwzględnić epokę w search haystack.

## Podpisane akcje w wierszu (pojazdy i składy)

Zamiast samych `IconButton` + tooltip — **`Button` `size="small"`** z `startIcon` i widocznym tekstem (istniejące klucze i18n):

- Dodaj do makiety / usuń z makiety (`list.actions.addToLayout`, `roster.removeButton`)
- Wypożycz (`rentals:granted.lend`)
- **Edytuj funkcje** (`list.actions.editFunctions`, ikona `Tune`) — **przenieść z Moje pojazdy na scaloną listę pojazdów**; widoczne dla właściciela / admina (jak edit/delete); nawigacja do `/my/vehicles/:id/functions`
- Edytuj / Usuń (`list.actions.edit` / `delete`)

Na składach: te same wzorce bez „Edytuj funkcje”. `variant="text"` lub `outlined`, `flexWrap` na wąskich ekranach.

## Frontend: usunięcie `useMyVehicles` / `useMyTrains`

Hooki wołają wyłącznie `GET /api/v1/vehicles` i `GET /api/v1/trains`. Po usunięciu stron My* nadal używane w:

- [`TrainDialog.tsx`](bigfred/web/src/components/TrainDialog.tsx) — lista pojazdów do składu → `useVehicleCatalogue(layoutId)` + filtr `ownerId === me.id` (ew. admin: własne + potrzebne do edycji cudzego składu — zachować możliwość wyboru pojazdów właściciela składu / własne).
- [`VehicleFunctionsPage.tsx`](bigfred/web/src/pages/VehicleFunctionsPage.tsx) — lookup pojazdu po id → `useVehicleCatalogue(layoutId)` (wymaga `useMe()` dla `layoutId`; `CatalogueVehicle` ma `name`/`epoch`/`kind` więc subtitle działa); przycisk „Wstecz” → `/my/vehicles`.

Potem usunąć `useMyVehicles` / `useMyTrains` z [`vehicles.ts`](bigfred/web/src/api/vehicles.ts).

**Uwaga TrainDialog:** przy tworzeniu składu użytkownik wybiera **własne** pojazdy — katalog z filtrem właściciela wystarczy. Przy edycji cudzego składu (admin) lista członków już jest w składzie; picker nowych członków: pojazdy właściciela składu albo wszystkie — **konkret: filtruj catalogue do `ownerId === editingTrain.ownerId` gdy edit, inaczej `me.id`**.

## Backend: uprzątnięcie endpointów „Moje”

Usunąć wyłącznie listę „owned-only” (strony Moje). **Zostawić** create/update/delete oraz catalogue:

| Usunąć | Zostawić |
|--------|----------|
| `GET /api/v1/vehicles` (`VehicleHandler.List`) | `GET /vehicles/catalogue`, `POST /vehicles`, `PUT/DELETE /vehicles/{id}`, by-external-id |
| `GET /api/v1/trains` (`TrainHandler.List`) | `GET /trains/catalogue`, `POST /trains`, `PUT/DELETE /trains/{id}`, patch members |

Warstwy do usunięcia, jeśli nic innego ich nie woła (stan dziś: tylko te handlery):

- [`cmd/vehicle.go`](bigfred/pkgs/bigfred/server/cmd/vehicle.go) — `ListOwned`
- [`cmd/train.go`](bigfred/pkgs/bigfred/server/cmd/train.go) — `ListOwned`
- [`repo/vehicles.go`](bigfred/pkgs/bigfred/server/repo/vehicles.go) — `ListByOwner` (nie mylić z lease `ListByOwner`)
- [`repo/trains.go`](bigfred/pkgs/bigfred/server/repo/trains.go) — `ListByOwner`
- Trasy w [`router.go`](bigfred/pkgs/bigfred/server/http/router.go)
- Testy unit pod `ListOwned` / HTTP List, jeśli istnieją

`ListCatalogue` / `ListAll` pozostają źródłem prawdy dla UI.

## Help (FAB) — treści z „Moje pojazdy” / „Moje składy”

Pływający przycisk pomocy ma **zostać** na scalonych podstronach z **tą samą treścią**, co dziś na Moje:

| Ścieżka kanoniczna | Wpis w [`helpRegistry.tsx`](bigfred/web/src/components/help/helpRegistry.tsx) | Klucz i18n |
|--------------------|-------------------------------------------------------------------------------|------------|
| `/my/vehicles` | `i18nKey: "myVehicles"` + `addIcon` / `functionsIcon` | `help.json` → `myVehicles` |
| `/my/trains` | `i18nKey: "myTrains"` | `help.json` → `myTrains` |

Ponieważ Available* renderujemy pod `/my/vehicles` i `/my/trains` (a `/fleet/*` tylko redirectuje), istniejące wpisy registry **nie wymagają przenosin** — nie dodawać osobnego helpa dla fleet, nie usuwać `myVehicles`/`myTrains`, nie zmieniać copy.

Weryfikacja po wdrożeniu: FAB widoczny na obu podstronach; dialog z ikonkami jak wcześniej; Menu → Pomoc nadal włącza ukryty FAB.

## Linki wewnętrzne
- [`RosterSection.tsx`](bigfred/web/src/components/RosterSection.tsx): manage → `/my/vehicles`, `/my/trains`.

## i18n
- `vehicle:catalogue.showOnlyMine` / `trainCatalogue.showOnlyMine` (pl/en/de).
- `vehicle:catalogue.filterKind` / `filterEpoch` / `filterAll` / `filterNoEpoch` (pl/en/de).
- `vehicle:catalogue.epochChip`: „Epoka {{epoch}}” / „Epoch {{epoch}}” / „Epoche {{epoch}}”.
- Reuse `list.refreshButton`, `list.addButton`, `trainList.addButton` oraz `vehicle:kind.*` dla etykiet typów.
- Usunąć nieużywane `nav.my.vehicles` / `nav.my.trains`; etykiety menu z `nav.fleet.available*`.
- Opcjonalnie usunąć lub ograniczyć martwe stringi `list.intro` / `trainList.intro` jeśli nikt ich nie renderuje.

## Poza zakresem
- Przebudowa tabeli składów do layoutu 2-kolumnowego jak pojazdy.
- Zmiana semantyki `POST /vehicles` / catalogue (bez nowych endpointów).
