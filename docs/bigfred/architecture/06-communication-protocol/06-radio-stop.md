### 4.6 Radio Stop – layout-wide emergency halt

**Radio Stop** (*radiostop*) is a layout-wide emergency signal modelled on
the real railway practice where a driver broadcasts an immediate halt
instruction over the radio. In BigFred it is distinct from:

- **`system.estop`** (§4.2) – brakes only the vehicles the **calling
  session** is actively driving on the **currently selected command
  station**;
- the dead-man's switch emergency plan (§4.5) – fires automatically when
  a user's last session is lost;
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
4. The action is **audited** as `system.radio_stop` (§3a.5).

Vehicles already at standstill (cached speed 0) are still included in
the audit row but may be skipped on the wire to avoid spurious speed-1
frames (same rule as manual `system.estop` in §7e.5).

#### 4.6.2 Authorization

Any authenticated user who **may drive at least one vehicle or train**
in the active layout may trigger Radio Stop:

- a **driver** on an owned or leased vehicle;
- a **signalman** while holding active takeover authority on a target;
- a user with a **temporary `driver` grant** that covers at least one
  roster vehicle.

Users who cannot open throttle mode (no drive scope) cannot trigger
Radio Stop. **`admin` alone is not sufficient** – the permanent admin
role does not imply drive rights (§7a.5). Admins who also hold
`driver` (or are mid-takeover as signalman) follow the same rule as
everyone else.

The check is implemented once in `RadioStopSecurityContext.CanTrigger`
(§7a.3) and reused by the WS handler, MCP tool surface and any future
REST alias.

#### 4.6.3 UI affordance

Radio Stop is exposed as a **dedicated button in the throttle overlay**
(§6.3b), separate from the per-session emergency brake
(`system.estop`). Placement: the throttle cockpit toolbar, visually
distinct (e.g. red, radio-handset icon) so it is not confused with
the narrower estop control.

- Label (PL): **„Radiostop”**; tooltip explains that the signal halts
  **all** locomotives on the layout and sounds the alarm on every
  throttle.
- The button is shown whenever throttle mode is open and the user
  passes the authorization rule above (same gate as the AppBar
  **Throttle** toggle in §6.3b).
- Tapping the button opens a **confirmation dialog** (destructive
  action) before the WS frame is sent.

Strings live in `throttle.json` (`throttle.radioStop.*`).

#### 4.6.4 Cross-process coordination

Radio Stop is a **layout-level** action; a layout may span multiple
command stations (§3a.4). `loco-server` owns the orchestration:

1. Client sends `system.radioStop` `{}` on the **control-plane**
   WebSocket (`/api/v1/ws`).
2. `loco-server` validates `RadioStopSecurityContext.CanTrigger`,
   interrupts scripts (`ScriptService.StopAllForLayout`), then fans
   out a control command to **every running `dcc-bus` daemon** for
   the layout (Redis pub/sub on
   `bigfred:layout:<L>:radio_stop`, same fan-out pattern as
   `bigfred:layout:<L>:emergency:<userId>` in §4.5.3b).
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
