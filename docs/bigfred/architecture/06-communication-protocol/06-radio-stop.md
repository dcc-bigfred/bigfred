### 4.6 Radio Stop – layout-wide emergency halt

**Radio Stop** (*radiostop*) is a layout-wide emergency signal modelled on
the real railway practice where a driver broadcasts an immediate halt
instruction over the radio. In BigFred it is distinct from:

- **`system.estop`** (§4.2) – brakes only the vehicles the **calling
  session** is actively driving on the **currently selected command
  station**;
- the dead-man's switch emergency plan (§4.5) – **fires automatically**
  when a user's last session is lost. Radio Stop is **manually**
  triggered and broader, but it deliberately **reuses the same
  emergency-plan execution path** for every connected driver as one of
  its two effects (§4.6.1a);
- **`estop_all`** in the emergency plan (§4.5.3) – **admin-only** and
  cuts track power on the command station;
- the walkie-talkie phrase `STOP_IMMEDIATELY` (§4.2, §3a.1) – a
  point-to-point radio message between a signalman and a single driver,
  with no braking side effect.

Radio Stop is a **deliberate, human-triggered, layout-scoped halt** with
audible feedback on every open throttle session.

#### 4.6.1 Behaviour

When a user triggers Radio Stop:

1. **Every drivable vehicle on the layout roster** receives a DCC
   emergency stop (`SetSpeed` with the EMG-stop bit, speed step 1 on
   the wire) on **every command station** attached to the layout,
   regardless of who is currently driving it or which command station
   their session has picked.
2. **Every open throttle session** in the layout (any user, any
   command-station pick) receives a `system.radioStop` push event and
   **plays the radiostop sound** locally. The sound is a bundled UI
   asset (not a DCC function); it is the same clip on every client so
   all operators hear the alarm simultaneously.
3. Running scripts owned by any user on affected vehicles are
   interrupted with reason `"radio_stop"` (same class of side effect as
   the dead-man's switch path in §4.5.3a).
4. **Every connected driver's dead-man's-switch emergency plan is
   fired** as a second, coordinated effect (§4.6.1a), so the same
   fail-safe machinery a lost session would trigger runs deliberately
   for everyone at once.
5. The action is **audited** as `system.radio_stop` (§3a.5).

Vehicles already at standstill (cached speed 0) are still included in
the audit row but may be skipped on the wire to avoid spurious speed-1
frames (same rule as manual `system.estop` in §7e.5).

#### 4.6.1a Hybrid execution – roster halt **and** per-user dead-man's plan

Radio Stop is implemented as **two coordinated effects**, not one:

- **(a) Roster-wide DCC emergency stop.** Every drivable vehicle on the
  layout roster is braked on every attached command station (§4.6.1
  step 1), regardless of whether anyone is currently driving it. This
  covers vehicles that are powered on the track but have **no live
  driver** (idle locomotives, abandoned sessions, vehicles moved by an
  external physical throttle).
- **(b) Per-user dead-man's-switch emergency plan.** For every user who
  has an open session in the layout with active `DriveTargets`,
  `loco-server` runs that user's persisted emergency plan through the
  **exact same path as §4.5.3** (`ScriptService.StopAllForUser` first,
  then the plan's `SetSpeed(0)` fan-out / lease handling). This means a
  driver's `release_my_leases` preference still revokes their outbound
  leases, and their running scripts are interrupted before the speed
  fan-out, so a sleeping `sleep(60)` script cannot re-issue speed after
  the halt.

The two effects are complementary: (a) guarantees the **track** is
quiet even for un-driven vehicles; (b) guarantees each **operator's**
own fail-safe (scripts, leases) is honoured exactly as if their last
session had just dropped.

**Two guardrails keep Radio Stop a *halt*, never a power escalation:**

- `estop_all` (admin-only track-power cut, §4.5.3) is **never
  auto-invoked** by Radio Stop. A connected admin whose personal plan
  is `estop_all` is **clamped to `stop_my_vehicles`** on the
  radiostop-initiated path. Track power is never cut as a side effect
  of a driver pressing Radio Stop — that would let any driver
  indirectly trigger an admin-only action, violating §7a.5.
- `none` is **upgraded to `stop_my_vehicles`** on this path. The whole
  point is to stop; `StopAllForUser` still runs so scripts cannot
  re-issue speed, and effect (a) already covers the vehicles on the
  wire.

In other words, the radiostop path runs each user's plan **clamped to
the `[stop_my_vehicles, release_my_leases]` band**.

#### 4.6.2 Authorization

Two independent grounds authorize Radio Stop in the active layout:

1. **Drive scope** — any user who **may drive at least one vehicle or
   train**:
   - a **driver** on an owned or leased vehicle;
   - a user with a **temporary `driver` grant** that covers at least one
     roster vehicle.
2. **Signalman role** — any user who holds the **`signalman` role** in
   the active layout (layout-scoped grant or self-grant). This is
   **independent of whether they currently occupy an interlocking or
   hold a takeover**: a signalman directing traffic must be able to halt
   the layout in an emergency even when they are not driving anything.
   This is the explicit broadening over the earlier rule, which only let
   a signalman trigger Radio Stop *while holding active takeover
   authority*.

Users who satisfy **neither** ground cannot trigger Radio Stop.
**`admin` alone is still not sufficient** – the permanent admin role
implies neither drive rights nor the signalman role (§7a.5). Admins who
also hold `driver` or `signalman` follow the rules above.

The check is implemented once in `RadioStopSecurityContext.CanTrigger`
(§7a.3) and reused by the WS handler, MCP tool surface and any future
REST alias. Its inputs are therefore the drivable roster **and** the
caller's effective roles in the layout (so the signalman ground can be
evaluated); `Allow` if either ground passes.

#### 4.6.3 UI affordance

Radio Stop is exposed as a **dedicated button on the throttle
overlay's left toolbar** (§6.3b), separate from the per-session
emergency brake (`system.estop`).

**Placement.** The throttle overlay carries a vertical/horizontal
toolbar pinned to the **left edge** of the driving surface. Its first
control is the **Fullscreen toggle** (browser Fullscreen API on the
overlay container); the **Radio Stop** button sits **immediately to
its right**. The Radio Stop button is visually distinct (red, with a
radio-handset / `RadioButtonChecked` icon) so it is never confused
with the narrower per-vehicle estop control.

- Icon: a radio handset (e.g. MUI `SettingsInputAntenna` / a radio
  glyph); colour `error` (red).
- Label / tooltip (PL): **„Radiostop”**; tooltip explains that the
  signal halts **all** locomotives on the layout and sounds the alarm
  on every throttle.
- The button is shown whenever throttle mode is open and the user
  passes the authorization rule above (same gate as the AppBar
  **Throttle** toggle in §6.3b).

**Second placement — interlocking view.** The staffed **interlocking
view** (§6.3d) also surfaces Radio Stop, **above its two panels**, so a
signalman can halt the layout without entering throttle mode. It reuses
the same component and confirm overlay but renders the **icon together
with the text label „Radio stop"** (the throttle placement is
icon-only). It is shown to any signalman staffing the box per the
signalman ground in §4.6.2.

**Confirmation overlay (destructive action).** Tapping Radio Stop does
**not** fire immediately. It opens a **modal overlay centred on the
screen** (MUI `Dialog`/`Backdrop`) containing, stacked vertically:

1. a primary **red** button **„Uruchom radiostop”** (*Trigger radio
   stop*) — sends `system.radioStop {}` on the control plane and closes
   the overlay;
2. below it, a neutral **„Anuluj”** (*Cancel*) button — dismisses the
   overlay with no side effect.

Only **„Uruchom radiostop”** emits the WS frame; the dialog is the sole
guard against accidental layout-wide halts.

**Audible feedback.** On receipt of the `system.radioStop` push event
(§4.6.5), every open throttle session plays the bundled alarm asset
served at **`/sounds/radiostop.ogg`**. It is a static UI asset (not a
DCC function and not locale-dependent); the same clip plays on every
client so all operators hear the alarm simultaneously. Playback is
best-effort: browsers that block autoplay until a user gesture will
still have one (the operator who pressed the button), and the alarm is
unlocked for the rest of the session on first interaction.

Strings live in `throttle.json` (`throttle.radioStop.*`,
`throttle.fullscreen.*`).

#### 4.6.4 Cross-process coordination

Radio Stop is a **layout-level** action; a layout may span multiple
command stations (§3a.4). `loco-server` owns the orchestration:

1. Client sends `system.radioStop` `{}` on the **control-plane**
   WebSocket (`/api/v1/ws`).
2. `loco-server` validates `RadioStopSecurityContext.CanTrigger`, then:
   - interrupts **all** running scripts on the layout
     (`ScriptService.StopAllForLayout`, reason `"radio_stop"`) so
     no script can race the halt regardless of owner;
   - runs the **per-user dead-man's plan** (§4.6.1a effect b) for every
     connected user in the layout that has active `DriveTargets`, via
     the existing emergency-plan executor (action **clamped** to the
     `[stop_my_vehicles, release_my_leases]` band);
   - fans out a control command to **every running `dcc-bus` daemon**
     for the layout (Redis pub/sub on `bigfred:layout:<L>:radio_stop`,
     same fan-out pattern as `bigfred:layout:<L>:emergency:<userId>`
     in §4.5.3b) to cover the **roster** (§4.6.1a effect a).
3. Each `dcc-bus` runs its local `applyEStopAll` against the vehicles
   on **its** command station and publishes affected addresses on
   `dcc-bus:evt:<L>:<C>`.
4. `loco-server` aggregates the per-station results, writes the audit
   row, and broadcasts `system.radioStop` to **every control-plane
   session** in the layout (not only throttle sessions – the event is
   harmless on the dashboard; clients without an open throttle overlay
   ignore the audio hook).

Debounce: at most one Radio Stop per layout per **2 s** so a
double-tap or two operators pressing simultaneously do not stampede
the command stations.

#### 4.6.5 WebSocket message types

Client → Server (control plane only):

- `system.radioStop` `{}` – request a layout-wide halt. Requires
  drive scope (§4.6.2). Acknowledged with the standard request-id
  envelope; on success the server fans out as above.

Server → Client (control plane, every session in the layout):

- `system.radioStop` `{ triggeredBy: { userId, login }, at }` –
  informational push after the halt has been issued. Throttle clients
  **must** play the radiostop sound on receipt; other surfaces may show
  a toast (`throttle.radioStop.toast`, interpolating `login`).

There is intentionally **no** `system.radioStop` action on the
`dcc-bus` data-plane WebSocket – the halt is never scoped to a single
command-station pick the way `system.estop` is.

#### 4.6.6 Relation to walkie-talkie radio

The walkie-talkie channel (§4.4) and Radio Stop solve different
problems. `STOP_IMMEDIATELY` is a **phrase** addressed to one user or
one interlocking; Radio Stop is a **system command** that brakes the
entire layout and sounds the alarm everywhere. A driver may use both in
the same operating session, but they do not subsume one another.
