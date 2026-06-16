# Implementation plan — Train announcements panel (M5.1)

Frontend-only slice: a static list of station PA messages in the
interlocking view, played locally on the signalman's device.

Specification:

- terminology — [`../architecture/00-terminology.md`](../architecture/00-terminology.md) (*train announcement*)
- frontend — [`../architecture/08-frontend-components.md`](../architecture/08-frontend-components.md) (§6.3d, right panel)
- acceptance — [`../architecture/14-acceptance-criteria/05-takeover-radio.md`](../architecture/14-acceptance-criteria/05-takeover-radio.md) (Train announcements)
- milestones — [`../architecture/13-delivery-order.md`](../architecture/13-delivery-order.md) (M5.1, step 18a)

## Starting point

- `pages/InterlockingPage.tsx` renders a **two-panel** staffed work area
  (radio chat + roster); a third column/tab slot is ready to add.
- Local audio patterns exist in `hooks/useRadioSounds.ts` and
  `hooks/useRadioInboundSound.ts`.
- **No backend work** — the announcement catalogue is a static TypeScript
  file shipped with the frontend bundle.

## Confirmed design decisions

1. **Local playback only** — clicking a row plays Ogg audio on the
   clicking browser tab. No WebSocket, Redis, SQLite or audit log.
2. **Static manifest** — `web/src/config/trainAnnouncements.ts` holds the
   catalogue. Keyed by interlocking **name** (stable across DB re-seeds;
   the panel already knows the name from the interlocking header fetch).
   A `"default"` key provides the fallback list.
3. **Stable `soundKey`** — kebab-case identifier maps 1:1 to
   `{soundKey}.ogg` under `/sounds/train-announcements/`.
4. **Labels via i18n** — manifest entries carry `labelKey`; UI resolves
   `t(\`trainAnnouncements:${labelKey}\`)`.
5. **Panel placement** — third column on wide screens (Radio | Składy |
   Zapowiedzi); third tab on narrow screens.
6. **Distinct from radio** — separate sound directory, no protocol overlap.

---

## Implementation stages

Two stages — both frontend.

```
Stage 1 ──► Stage 2
 panel +     assets +
 manifest    polish
```

| Stage | Focus | Delivers |
|-------|-------|----------|
| **1** | Panel + manifest + hook | Static config, `useTrainAnnouncementSound`, `<InterlockingTrainAnnouncementsPanel>`, three-column layout, i18n |
| **2** | Assets & acceptance | `.ogg` files, optional playing-row highlight, manual walkthrough |

---

### Stage 1 — Panel, manifest, hook

#### Static manifest

**`web/src/config/trainAnnouncements.ts`**

```ts
export type TrainAnnouncementEntry = {
  soundKey: string;
  labelKey: string;
};

/** Interlocking name → ordered announcement list. Use "default" as fallback. */
export const TRAIN_ANNOUNCEMENTS: Record<string, TrainAnnouncementEntry[]> = {
  default: [
    { soundKey: "track-1-freight", labelKey: "track1Freight" },
    { soundKey: "departure-gluszyca", labelKey: "departureGluszyca" },
    { soundKey: "departure-wroclaw", labelKey: "departureWroclaw" },
    { soundKey: "departure-warszawa-centralna", labelKey: "departureWarszawaCentralna" },
  ],
  // "Wałbrzych Główny": [ … dedicated list … ],
};

export function trainAnnouncementsFor(interlockingName: string): TrainAnnouncementEntry[] {
  return TRAIN_ANNOUNCEMENTS[interlockingName] ?? TRAIN_ANNOUNCEMENTS.default ?? [];
}
```

To customise a specific box later, add a named key alongside `"default"`.

#### Hook

**`web/src/hooks/useTrainAnnouncementSound.ts`**

- `play(soundKey: string)` — URL `/sounds/train-announcements/${soundKey}.ogg`,
  cache `HTMLAudioElement` per URL, pause in-flight clip before starting
  the next one.
- Swallow `play()` rejections silently (same as `useRadioSounds`).

#### Component

**`web/src/components/interlocking/InterlockingTrainAnnouncementsPanel.tsx`**

- Props: `interlockingName: string`.
- Calls `trainAnnouncementsFor(interlockingName)` — no fetch, no React Query.
- Layout mirrors `<InterlockingChatPanel>`: outlined `Paper`, title
  **„Zapowiedzi pociągów"**, scrollable `List`.
- Each row: `ListItemButton` + translated label + `VolumeUp` icon;
  `onClick` → `play(soundKey)`.
- Empty state when the resolved list is `[]`.

#### Page integration

**`pages/InterlockingPage.tsx`** (`InterlockingStaffedWorkArea`):

- Pass `interlockingName` from the interlocking detail already loaded by
  the page.
- Add third `Box` column with `<InterlockingTrainAnnouncementsPanel>`.
- Narrow screens: third tab `view.panels.announcements`, visible when
  `tab === 2`.

#### i18n

- **`web/src/i18n/locales/pl/trainAnnouncements.json`** — four example labels.
- **`web/src/i18n/locales/en/trainAnnouncements.json`** — English mirror.
- Extend **`interlocking.json`**: `view.panels.announcements`,
  `view.announcements.title`, `view.announcements.empty`.
- Register namespace in i18n config.

**Stage 1 done when:** staffed interlocking shows the third panel; clicking
a row triggers local playback (file may be missing yet).

---

### Stage 2 — Assets & polish

#### Assets

Add `.ogg` files under `web/public/sounds/train-announcements/` matching
the `soundKey` values in the manifest.

#### UX polish (optional)

- Highlight the active row while its audio plays.
- Verify re-click restarts from the beginning.
- Verify switching mid-playback stops the previous clip.

#### Manual acceptance walkthrough

1. Occupy an interlocking — third panel / tab visible.
2. Click each entry — audio **only on that device**.
3. Second browser on same box — clicks stay local.
4. Chat panel unaffected.
5. Rename interlocking to a name absent from manifest with no `"default"`
   key removed — empty-state hint (only if testing edge case).

---

## Out of scope

- Backend table, REST endpoint or admin CRUD.
- WebSocket broadcast to external PA hardware.
- Audit log entries.
- Runtime upload / transcode of Ogg files.

## File checklist

| Area | File |
|------|------|
| Config | `web/src/config/trainAnnouncements.ts` |
| Hook | `web/src/hooks/useTrainAnnouncementSound.ts` |
| Component | `web/src/components/interlocking/InterlockingTrainAnnouncementsPanel.tsx` |
| Page | `web/src/pages/InterlockingPage.tsx` |
| i18n | `web/src/i18n/locales/{pl,en}/trainAnnouncements.json` |
| i18n | `web/src/i18n/locales/{pl,en}/interlocking.json` (panel keys) |
| Assets | `web/public/sounds/train-announcements/*.ogg` |
