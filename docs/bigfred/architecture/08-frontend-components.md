## 6. Frontend Components

### 6.1 WebSocket Hook (`useSocket.ts`)

```ts
import { useEffect, useRef, useCallback } from "react";
import { useLocoStore } from "./store";

type Envelope = { type: string; id?: string; payload?: unknown };

export function useSocket(url: string) {
  const wsRef = useRef<WebSocket | null>(null);
  const applyEvent = useLocoStore((s) => s.applyEvent);

  const connect = useCallback(() => {
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onmessage = (e) => {
      const env: Envelope = JSON.parse(e.data);
      applyEvent(env);
    };
    ws.onclose = () => setTimeout(connect, 1000); // reconnect with backoff
  }, [url, applyEvent]);

  useEffect(() => {
    connect();
    return () => wsRef.current?.close();
  }, [connect]);

  const send = useCallback((env: Envelope) => {
    wsRef.current?.send(JSON.stringify(env));
  }, []);

  return { send };
}
```

### 6.2 Zustand Store for Locomotive State

```ts
import { create } from "zustand";

type LocoState = {
  addr: number;
  speed: number;
  forward: boolean;
  functions: number[];
};

type Store = {
  locos: Record<number, LocoState>;
  applyEvent: (env: { type: string; payload?: any }) => void;
};

export const useLocoStore = create<Store>((set) => ({
  locos: {},
  applyEvent: (env) => {
    if (env.type === "loco.state") {
      const st = env.payload as LocoState;
      set((s) => ({ locos: { ...s.locos, [st.addr]: st } }));
    }
  },
}));
```

### 6.3 Control Component (Material UI)

```tsx
import { useEffect } from "react";
import {
  Card,
  CardContent,
  CardActions,
  Typography,
  Slider,
  Stack,
  IconButton,
  ToggleButton,
  ToggleButtonGroup,
} from "@mui/material";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import ArrowForwardIcon from "@mui/icons-material/ArrowForward";
import StopIcon from "@mui/icons-material/Stop";

function LocoControl({ addr }: { addr: number }) {
  const { send } = useSocket(`ws://${location.host}/api/v1/ws`);
  const state = useLocoStore((s) => s.locos[addr]);

  useEffect(() => {
    send({ type: "loco.subscribe", payload: { addr } });
    return () => send({ type: "loco.unsubscribe", payload: { addr } });
  }, [addr, send]);

  const setSpeed = (speed: number) =>
    send({
      type: "loco.setSpeed",
      payload: { addr, speed, forward: state?.forward ?? true },
    });

  const setDirection = (forward: boolean) =>
    send({
      type: "loco.setSpeed",
      payload: { addr, speed: state?.speed ?? 0, forward },
    });

  return (
    <Card sx={{ maxWidth: 480, m: 2 }}>
      <CardContent>
        <Typography variant="h5" gutterBottom>
          Loco #{addr}
        </Typography>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          {state?.speed ?? 0} step{state?.forward ? " ▶" : " ◀"}
        </Typography>
        <Slider
          value={state?.speed ?? 0}
          min={0}
          max={127}
          aria-label="Throttle"
          onChange={(_, v) => setSpeed(v as number)}
        />
      </CardContent>
      <CardActions>
        <ToggleButtonGroup
          exclusive
          value={state?.forward ? "fwd" : "rev"}
          onChange={(_, v) => v && setDirection(v === "fwd")}
          size="small"
        >
          <ToggleButton value="rev" aria-label="Reverse">
            <ArrowBackIcon />
          </ToggleButton>
          <ToggleButton value="fwd" aria-label="Forward">
            <ArrowForwardIcon />
          </ToggleButton>
        </ToggleButtonGroup>
        <IconButton color="error" onClick={() => setSpeed(0)} aria-label="Stop">
          <StopIcon />
        </IconButton>
      </CardActions>
    </Card>
  );
}
```

### 6.3a Train Control View – the same slider, one fan-out

The train control view is intentionally the **single-vehicle view
with a different command on the slider**. It mounts
`<ThrottleSlider>` (same MUI `Slider` component, same range 0..126,
same direction toggle), but its `onChange` dispatches
`train.setSpeed` instead of `loco.setSpeed`, and it subscribes to
member state via `train.subscribe`. Function buttons and scripts
remain per-member: under the slider the page renders one
`<FunctionButtons>` + `<ScriptButtons>` row **per member vehicle**,
so the driver still controls horns, lights and shunting modes on
each loco independently.

```tsx
function TrainControl({ trainId }: { trainId: number }) {
  const { send } = useSocket(`ws://${location.host}/api/v1/ws`);
  const train = useTrain(trainId);              // REST: train + members
  const members = train.members ?? [];
  const memberStates = useLocoStore((s) =>
    members.map((m) => s.locos[m.vehicle.dccAddr]),
  );

  // The slider shows the train speed; we use the first non-reversed
  // member as the "speed witness" (members run in lock-step, modulo
  // the Reversed flip the server applies on its side).
  const witness = memberStates.find((_, i) => !members[i].reversed) ?? memberStates[0];
  const speed   = witness?.speed   ?? 0;
  const forward = witness?.forward ?? true;

  useEffect(() => {
    send({ type: "train.subscribe",   payload: { trainId } });
    return () => send({ type: "train.unsubscribe", payload: { trainId } });
  }, [trainId, send]);

  const onSpeedChange = (next: number) =>
    send({ type: "train.setSpeed", payload: { trainId, speed: next, forward } });

  const onDirectionChange = (nextForward: boolean) =>
    send({ type: "train.setSpeed", payload: { trainId, speed, forward: nextForward } });

  return (
    <Card sx={{ maxWidth: 720, m: 2 }}>
      <CardContent>
        <Typography variant="h5">{train.name}</Typography>

        {/* SAME slider component as LocoControl – this is the whole point. */}
        <ThrottleSlider value={speed} forward={forward}
                        onValueChange={onSpeedChange}
                        onDirectionChange={onDirectionChange} />

        {/* Per-member function / script rows below the shared slider. */}
        <Stack spacing={1} sx={{ mt: 2 }}>
          {members.map((m, i) => (
            <Stack key={m.id} direction="row" alignItems="center" spacing={1}>
              <Typography variant="body2" sx={{ minWidth: 96 }}>
                {m.vehicle.name} {m.reversed && <Chip size="small" label="rev" />}
              </Typography>
              <FunctionButtons vehicle={m.vehicle} />
              <ScriptButtons   vehicle={m.vehicle} />
              {memberStates[i] === undefined && <Chip size="small" color="warning" label="offline" />}
            </Stack>
          ))}
        </Stack>
      </CardContent>
    </Card>
  );
}
```

The slider is **not duplicated**: `ThrottleSlider.tsx` is the same
component used in `LocoControl`. The only thing that changes is the
WS message the parent component dispatches, and that single decision
keeps `LocoControlPage` and `TrainControlPage` visually identical
from the driver's standpoint.

### 6.3b Throttle mode – full-screen overlay

**Throttle mode** (*tryb sterowania „Throttle”*, see §1) is how a
**driver** or a **signalman** (after a granted **takeover**) operates a
**vehicle** or **train** in real time. It is not a separate application
route the user navigates away from: it is a layer above the rest of
BigFred that hosts the driving surface (`ThrottleSlider`, function and
script buttons, command-station picker, script console, takeover
banners, dead-man's switch affordances).

#### Entry, exit and shell layout

The sticky top **AppBar** in `AppShell.tsx` carries a **Throttle** button
(labelled via `vehicle.json`, icon: engineer / *maszynista*) among the
account-level controls. Clicking it toggles throttle mode:

- **Open** – a full-screen overlay is rendered immediately below the
  `AppBar`. It occupies the remaining viewport height (`position:
  fixed`, top aligned to the AppBar, high `z-index`). Every other page
  (admin screens, vehicle lists, future radio panel) stays mounted
  underneath but is visually covered; only the **AppBar remains visible**.
- **Close** – the same button (or an explicit close control on the
  overlay) dismisses the layer without tearing down WebSocket
  subscriptions, so re-entry is instant.

Gating: the button is shown to users who may drive at least one vehicle
or train in the current session layout (drivers on owned/leased scope;
signalmen only while they hold active takeover authority on a target).
Exact rules follow the permission matrix in §11.

#### Controls available inside the overlay

While throttle mode is open and driving authority is held, the operator
can:

- set speed and direction (`loco.setSpeed` / `train.setSpeed`),
- toggle registered DCC functions (`loco.toggleFn`),
- start and stop attached scripts (`script.run` / `script.stop`),
- trigger emergency braking (`system.estop`).

All of the above travel over the existing WebSocket connection (§4.2).

#### Server as source of truth (multi-pilot sync)

The DCC bus and the backend **command station** are shared state. A
physical throttle on the layout, another browser tab, or an external
API/MCP client may change the same locomotive while BigFred is open.
The overlay therefore **must not treat the last outbound command as
ground truth**. Instead it renders from server push events — principally
`loco.state` for speed, direction and the runtime `functions` array —
and re-fetches function definitions on `vehicle.functionsChanged`. When
an external pilot moves the speed step or flips a function, every open
throttle overlay subscribed to that address converges to the server
state within one polling/event round trip (see M1 acceptance criteria).

When a **takeover** is active, the affected **driver's** overlay for
that target becomes read-only telemetry (`controlledBy.kind ==
"signalman"`); the **signalman's** overlay receives full write access
until `takeover.released`.

#### Illustrative shell wiring

```tsx
// AppShell.tsx (excerpt) – Throttle toggle on the top bar
import EngineeringIcon from "@mui/icons-material/Engineering";
import { ThrottleOverlay } from "./ThrottleOverlay";

export function AppShell({ children }: { children: React.ReactNode }) {
  const [throttleOpen, setThrottleOpen] = useState(false);
  const canDrive = useCanDriveAny(); // owned, leased, or takeover-held scope

  return (
    <>
      <AppBar position="sticky">
        <Toolbar>
          <Typography variant="h6" sx={{ flexGrow: 1 }}>BigFred</Typography>
          {canDrive && (
            <IconButton
              color="inherit"
              aria-label={t("vehicle:throttle.open")}
              aria-pressed={throttleOpen}
              onClick={() => setThrottleOpen((v) => !v)}
            >
              <EngineeringIcon />
            </IconButton>
          )}
          {/* account / admin menus, locale toggle … */}
        </Toolbar>
      </AppBar>

      <main>{children}</main>

      {throttleOpen && (
        <ThrottleOverlay onClose={() => setThrottleOpen(false)} />
      )}
    </>
  );
}
```

`ThrottleOverlay` hosts vehicle/train selection and mounts
`LocoControlPage` / `TrainControlPage` content from §6.3 and §6.3a.

#### Dual-WebSocket model (after §7e ships)

When §7e is live, the overlay manages **two** independent WebSocket
connections (see §7e.7 for the full lifecycle):

1. **Control-plane WS** to `loco-server` (`/api/v1/ws`) — already
   open since login. Carries `session.*`, `takeover.*`, `radio.*`,
   `script.*`, `presence`, `auth.elevationChanged`, `train.*`, and
   the command-station picker (`session.setCommandStation` /
   `session.commandStationChanged`).
2. **Data-plane WS** to the picked `dcc-bus` daemon
   (`ws://host:<port>/ws?token=<jwt>`, returned via
   `session.opened.availableCommandStations[i].wsUrl`). Carries
   `loco.subscribe` / `loco.unsubscribe` / `loco.setSpeed` /
   `loco.toggleFn` / `system.estop` / `ping` only. Re-opened when
   the user switches command stations.

`<ThrottleSlider>` and `<FunctionButtons>` dispatch on the data
plane via a `useDataPlane()` hook; `<ScriptButtons>`, the takeover
banner and the radio panel keep using `useControlPlane()`. Selecting
the right socket is encapsulated; component code does **not** know
about ports. The Zustand store splits accordingly: a `dccBusUrl` /
`dccBusOpened` slice for the daemon connection, the existing
`session` slice for the server connection. Until §7e ships, both
hooks resolve to the same control-plane connection (the M1 baseline
is unchanged).

The command-station dropdown inside `<ThrottleHeader>` renders
`status` per row (`RUNNING` / `STOPPED` / `STARTING` / `DEGRADED`)
based on `availableCommandStations[i].status` and disables rows
whose `wsUrl == null` until the user selects them (selection
triggers daemon spawn). The `<SharedBusChip>` lights up when
`dcc-bus.opened.sharedBus === true` to surface §3a.4 rule 9 to the
driver.

### 6.3e Vehicle catalogue and function editor

Route for the owner's **vehicle catalogue** (*lista pojazdów / lokomotyw*):
`/vehicles` (`LocoListPage.tsx`). The page lists every vehicle the caller
owns (`GET /api/v1/vehicles`, filtered to `ownerUserId == me`), with columns
for kind, DCC address, name and number. Each row exposes two owner-only
actions in the trailing action column:

| Control | Icon (MUI) | Behaviour |
|---------|------------|-----------|
| **Edytuj** | `Edit` | Navigates to `/vehicles/{addr}/edit` — metadata form (name, kind, number, template pick, …) as today. |
| **Edytuj funkcje** | `Tune` (or `Functions`) | Navigates to `/vehicles/{addr}/functions` — the function-definition editor described below. Tooltip and `aria-label` come from `vehicle.json` (`vehicle.functions.edit`). |

Lessees and non-owners never see either action. Vehicles without a DCC
address (*dummy*) may still open the function editor (definitions are stored
for when an address is added later), but the throttle will not emit DCC for
them until `dccAddress` is set.

#### Function editor page (`VehicleFunctionsPage.tsx`)

Route: `/vehicles/{addr}/functions`. Header shows vehicle name and DCC
address; a back link returns to `/vehicles`.

The page edits the **resolved** function list for that vehicle
(`GET /api/v1/vehicles/{addr}/functions`). When `source: "template"` the UI
shows a read-only banner (“Lista dziedziczona ze szablonu …”) until the
first mutation, which triggers server-side copy-on-write (§3a.6).

**Adding a slot** — toolbar button **Dodaj funkcję** opens a dialog:

- **Numer** — pick an unused DCC slot from `F0`–`F31` (dropdown of free
  numbers only).
- **Tytuł** — free-text label shown on the throttle button and in tooltips
  (`name` field on the wire).
- **Ikona** — visual picker grid populated from
  `GET /api/v1/function-icons` (closed catalogue in
  [§3a.8](./05-domain-model/08-function-icon-catalogue.md)); choosing an
  icon while **Tytuł** is empty copies the icon label into the title field.

Confirming calls `PUT …/functions/{num}`.

**Editing** — each list row is editable for title and icon;
changes debounce to the same `PUT` endpoint.

**Removing** — row action **Usuń** → `DELETE …/functions/{num}`.

**Reordering** — the list is a drag-and-drop sortable (`@dnd-kit` or
equivalent). On drop the client posts
`POST …/functions/reorder { positions: [{ num, position }, …] }`.
`position` is dense `0..n-1` in display order.

The list is sorted by `position` ascending at all times. **The same order
is used in throttle mode**: `<FunctionButtons>` renders one button per
registered function, left-to-right / top-to-bottom in `position` order.
Reordering on this page therefore immediately changes how the driver sees
functions in the **Throttle** overlay (after refetch or
`vehicle.functionsChanged`).

**Throttle visibility** — every function row the owner registered for this
vehicle appears in `<FunctionButtons>` for that vehicle inside throttle mode
(§6.3b). There is no separate “favourites” subset: the catalogue on this page
*is* the throttle button row (scripts from §6.7 still append after the
function buttons). Lessees and signalmen with driving authority see the same
buttons but cannot open this editor.

```tsx
// VehicleFunctionsPage.tsx (structure sketch)
function VehicleFunctionsPage() {
  const { addr } = useParams();
  const { data: fns = [], refetch } = useQuery({
    queryKey: ["vehicle-functions", addr],
    queryFn: () => fetch(`/api/v1/vehicles/${addr}/functions`).then((r) => r.json()),
  });
  const icons = useFunctionIcons(); // GET /api/v1/function-icons, cached

  const onReorder = (ordered: ResolvedFunction[]) =>
    fetch(`/api/v1/vehicles/${addr}/functions/reorder`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        positions: ordered.map((f, i) => ({ num: f.num, position: i })),
      }),
    }).then(() => refetch());

  return (
    <Container>
      <FunctionList
        items={fns.sort((a, b) => a.position - b.position)}
        icons={icons}
        onReorder={onReorder}
        onSave={(f) => putFunction(addr, f)}
        onDelete={(num) => deleteFunction(addr, num)}
      />
    </Container>
  );
}
```

`FunctionButtons.tsx` already sorts by `position` when rendering; no
second sort key is applied in the throttle.

### 6.3c Layout dashboard (`HomePage`)

After login the default route is `/` (`HomePage.tsx`). This **dashboard**
(*pulpit makiety*, see §1) is the operational home screen for the
layout the user picked on the login form. It renders **three MUI
`DataGrid` / `Table` panels** stacked vertically (or tabbed on very
small screens), each fed by REST on mount and kept fresh by WebSocket
fan-out (§4.2).

#### 1. Layout vehicle roster

Default view: vehicles **added to this layout** (`GET
/api/v1/layouts/{layoutId}/vehicles`). Columns include at minimum DCC
address, name, owner login.

Toolbar actions (i18n keys in `home.json` / `vehicle.json`):

- **Pokaż moje pojazdy** (*Show my vehicles*) – toggles the table
  between the shared roster and the caller's own catalogue (`GET
  …/vehicles/mine`), marking which rows are already on the layout
  (`onLayout: true`). Lets a driver review their fleet without losing
  layout context.
- **Dodaj mój pojazd do makiety** (*Add my vehicle to layout*) –
  opens a picker dialog listing owned vehicles not yet on the roster;
  confirming fires `POST …/vehicles { vehicleAddr }`. Only vehicles
  the user **owns** may be added; leased vehicles are excluded.

Row actions (owner only): remove from layout (`DELETE …/vehicles/{addr}`).

#### 2. Online users

Live table of everyone currently connected to the layout (`GET
/api/v1/layouts/{layoutId}/presence`, updated on
`layout.presenceChanged`). Columns:

| Column | Content |
|--------|---------|
| Login | `login` |
| Role | effective role in this layout (`driver` / `signalman` / `admin`, via `role` namespace) |
| Interlocking | if the user occupies a signal box: interlocking name; otherwise em dash |

One row per **user**, not per tab – multiple WS sessions from the same
login collapse into a single row.

#### 3. Interlockings

Table of interlockings whitelisted in this layout (`GET
/api/v1/interlockings`, enriched with `occupant`). Columns: name,
location, **Obstawia** (*staffed by* – occupant login or "wolna" /
vacant). Rows are **clickable**: navigation to `/interlockings/:id`
(`InterlockingPage`).

All three panels share the active `layoutId` from `useMe()`; there is
no layout switcher on this page (layout is immutable for the session).

```tsx
// HomePage.tsx (structure sketch)
function HomePage() {
  const me = useMe().data!;
  const layoutId = me.layoutId;
  const [showMine, setShowMine] = useState(false);

  const vehicles = useQuery({
    queryKey: ["layout-vehicles", layoutId, showMine],
    queryFn: () =>
      fetch(
        showMine
          ? `/api/v1/layouts/${layoutId}/vehicles/mine`
          : `/api/v1/layouts/${layoutId}/vehicles`,
      ).then((r) => r.json()),
  });
  // presence + interlockings analogous; useSocket merges WS events

  return (
    <Container maxWidth="lg">
      <LayoutVehiclesTable data={vehicles.data} showMine={showMine}
        onToggleMine={() => setShowMine((v) => !v)} layoutId={layoutId} />
      <OnlineUsersTable layoutId={layoutId} />
      <InterlockingsTable layoutId={layoutId} onRowClick={(id) => navigate(`/interlockings/${id}`)} />
    </Container>
  );
}
```

### 6.3d Interlocking view and occupation

Route: `/interlockings/:id` (`InterlockingPage.tsx`). Opened from the
dashboard interlockings table (§6.3c) or via direct link. Visible to
every authenticated user in the layout; **occupation controls** are
enabled only for users with the layout-scoped **signalman** role.

#### Layout of the page

1. **Header** – interlocking name, location, current occupant (live via
   `interlocking.occupantChanged`).
2. **Action bar** (signalmen only):
   - **Obsadź nastawnię** (*Occupy interlocking*) – visible when the
     caller is **not** the active occupant. Calls
     `POST /api/v1/interlockings/{id}/join`. If the box is vacant the
     join succeeds immediately. If another signalman is already
     staffing it, the UI shows a **confirmation dialog** naming the
     incumbent and explaining that they will be displaced; on confirm
     the client retries with `{ force: true }`. This prevents a
     forgotten session from blocking the interlocking indefinitely
     while still requiring an explicit human decision.
   - **Opuść nastawnię** (*Leave interlocking*) – visible when the
     caller **is** the active occupant. Calls
     `POST /api/v1/interlockings/{id}/leave`.
3. **Radio panel** – the interlocking's **chat** with drivers and other
   signal boxes: a scrollable message list (`radio.message` events +
   persisted replay on mount) and a phrase picker that emits
   `radio.send`. Traffic addressed to this interlocking's id, messages
   from/to drivers in the layout, and cross-interlocking phrases the
   protocol allows are all rendered here – this is the signalman's
   primary comms surface while staffing the box.

#### Leaving the view while still occupying

If the active occupant navigates away from `/interlockings/:id` (back
to the dashboard, admin page, browser back, …) while still holding an
`InterlockingSession`, the router **blocks** the transition and shows a
dialog:

> You are staffing this interlocking. Leave the interlocking?

- **Confirm** – `POST …/leave`, then proceed with navigation.
- **Cancel** – stay on the interlocking view.

Implementation: React Router `useBlocker` (or equivalent) keyed off
"am I the occupant?" local state synced from REST + WS. Closing the
browser tab does **not** auto-leave (the session stays until explicit
leave, displacement, or logout) – only in-app navigation triggers the
prompt.

#### Displaced occupant UX

When `interlocking.occupantChanged { reason:"displaced" }` targets the
current user, show a non-blocking toast, clear occupation state, and
disable takeover/radio actions that require active occupation until
they re-join or navigate away.

```tsx
// InterlockingPage.tsx (occupation hook sketch)
function useInterlockingOccupation(interlockingId: number) {
  const me = useMe().data!;
  const isSignalman = /* effective role in layout includes signalman */;

  const join = async (force = false) => {
    const res = await fetch(`/api/v1/interlockings/${interlockingId}/join`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ force }),
    });
    if (res.status === 409 && !force) {
      const incumbent = await res.json(); // { occupant: { login } }
      const ok = await confirmDisplaceDialog(incumbent);
      if (ok) return join(true);
      return;
    }
    // refresh local occupant state …
  };

  // useBlocker: when isOccupying && navigating away → leave dialog
  return { isSignalman, join, leave, isOccupying, … };
}
```

### 6.4 MUI Setup – Theme, Roboto Font, App Shell

Following [MUI's installation guide](https://mui.com/material-ui/getting-started/installation/),
install the core package, the styled-engine, the icons package, and the
Roboto font:

```bash
npm install @mui/material @emotion/react @emotion/styled
npm install @mui/icons-material
npm install @fontsource/roboto
```

`src/theme.ts` – central theme configuration. Material UI ships with
sensible defaults and a responsive 12-column grid; here we just tweak
palette and breakpoints to suit a throttle-style UI that must work on
small touchscreens:

```ts
import { createTheme } from "@mui/material/styles";

export const theme = createTheme({
  palette: {
    mode: "dark", // a command station console is easier to read in dark mode
    primary: { main: "#90caf9" },
    error: { main: "#ef5350" },
  },
  shape: { borderRadius: 12 },
  components: {
    MuiSlider: {
      styleOverrides: {
        thumb: { width: 28, height: 28 }, // larger touch targets on phones
      },
    },
  },
});
```

`src/main.tsx` – wire up `ThemeProvider` + `CssBaseline` (CSS reset) and
the Roboto font once at the root:

```tsx
import "@fontsource/roboto/300.css";
import "@fontsource/roboto/400.css";
import "@fontsource/roboto/500.css";
import "@fontsource/roboto/700.css";

import { createRoot } from "react-dom/client";
import { ThemeProvider, CssBaseline } from "@mui/material";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { theme } from "./theme";
import { App } from "./App";

const queryClient = new QueryClient();

createRoot(document.getElementById("root")!).render(
  <ThemeProvider theme={theme}>
    <CssBaseline />
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </ThemeProvider>,
);
```

`src/components/AppShell.tsx` – top-level navigation that adapts to
phone vs. desktop via MUI's `useMediaQuery` and breakpoint system:

```tsx
import { AppBar, Toolbar, Typography, IconButton, Drawer, useMediaQuery, useTheme } from "@mui/material";
import MenuIcon from "@mui/icons-material/Menu";
import { useState } from "react";

export function AppShell({ children }: { children: React.ReactNode }) {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("md"));
  const [open, setOpen] = useState(!isMobile);

  return (
    <>
      <AppBar position="sticky">
        <Toolbar>
          {isMobile && (
            <IconButton color="inherit" onClick={() => setOpen((v) => !v)} edge="start">
              <MenuIcon />
            </IconButton>
          )}
          <Typography variant="h6">BigFred Control</Typography>
        </Toolbar>
      </AppBar>
      <Drawer
        variant={isMobile ? "temporary" : "permanent"}
        open={open}
        onClose={() => setOpen(false)}
      >
        {/* loco list / nav */}
      </Drawer>
      <main>{children}</main>
    </>
  );
}
```

### 6.5 Why Material UI Fits This Project

- **Accessibility out of the box.** `Slider`, `ToggleButton`, `IconButton`
  and friends ship with proper ARIA attributes, keyboard handling and
  focus management. This matters when the app is used on a phone with
  voice-over enabled or with a hardware keyboard.
- **Responsive primitives.** `Grid`, `Stack`, `useMediaQuery` and the
  `sx` prop make it trivial to render the same `LocoCard` as a wide row
  on desktop and as a single-column stack on a phone, without writing
  custom CSS.
- **Theming.** A single `createTheme` call defines colors, spacing,
  typography and touch-target sizes globally. Dark mode for a control
  room is a one-line switch.
- **Icon coverage.** `@mui/icons-material` exposes the full Material
  Symbols catalogue, which already contains everything a model railway
  UI needs (`PlayArrow`, `Stop`, `Lightbulb`, `VolumeUp`, `Settings`,
  `Power`, etc.) – no separate icon library required.
- **Maturity.** MUI is the largest React UI library; long-term support
  and community size reduce the risk of an unmaintained dependency in a
  hobby-but-long-lived project. See [MUI Overview](https://mui.com/material-ui/getting-started/).

### 6.6 REST via TanStack Query (List / Edit)

```ts
export const useLocos = () =>
  useQuery({
    queryKey: ["locos"],
    queryFn: () => fetch("/api/v1/locos").then((r) => r.json()),
  });
```

### 6.7 Script Buttons and Console (browser side)

With execution moved to the server (§3a.7), the **frontend's job is
trivial**: render a button per attached script that emits
`script.run` / `script.stop`, and a console pane that subscribes to
`script.log` events for the currently-displayed throttle. No
PyScript, no Web Worker, no Python source files. Goja runs on the
server; the browser just operates the play/stop button.

```tsx
// ScriptButtons.tsx
function ScriptButtons({ vehicle }: { vehicle: Vehicle }) {
  const { data: scripts = [] } = useQuery({
    queryKey: ["vehicle-scripts", vehicle.addr],
    queryFn: () => fetch(`/api/v1/vehicles/${vehicle.addr}/scripts`).then(r => r.json()),
  });
  const { send } = useSocket();
  const activeRuns = useScriptStore((s) => s.activeRuns); // map<attachmentId, runId>

  return (
    <Stack direction="row" spacing={1}>
      {scripts.map((s) => {
        const runId = activeRuns[s.attachmentId];
        const running = !!runId;
        return (
          <IconButton
            key={s.attachmentId}
            color={running ? "secondary" : "primary"}
            onClick={() => {
              if (running) send({ type: "script.stop", payload: { runId } });
              else send({ type: "script.run",  payload: { scriptId: s.id, attachmentId: s.attachmentId } });
            }}
          >
            <FunctionIcon name={s.icon} />
          </IconButton>
        );
      })}
    </Stack>
  );
}
```

`useScriptStore` is a tiny Zustand slice that listens for
`script.runStarted` / `script.runStopped` events on the existing WS
and keeps `activeRuns[attachmentId] = runId`. That's the entire
client-side state. **Stop** on the phone is just
`send({ type:"script.stop", payload:{ runId } })` – the server
forwards it to the executor, which interrupts the VM.

`ScriptConsole.tsx` is a `<List>` that subscribes to `script.log`
events for the active throttle's `runId` and `script.runStopped`
events to flush the buffer with the final `{ reason, durationMs }`
line. The editor (`ScriptEditor.tsx`) on the Scripts page uses
`@monaco-editor/react` with `language="javascript"`, posts the
edited source via `PUT /api/v1/scripts/{id}`, and otherwise does
nothing executable.

### 6.8 Internationalization (pointer)

Every user-visible string in the components above (button labels,
error toasts, table headers, plural counters) is rendered through
`react-i18next` with namespace catalogues bundled into `web/dist`.
Backend codes (`ApiError.code`, `RadioPhrase`, `FunctionIcon`,
`AuditAction`, …) map 1:1 to translation keys; user-entered names
and audit-log denormalized snapshots are rendered verbatim. The
`I18nextProvider` wraps the app **above** `ThemeProvider` and
`QueryClientProvider` in `main.tsx`. The full specification —
namespace layout, key naming, plural rules, locale persistence,
type-safe key generation — lives in [§7c i18n](./09a-i18n.md).
Components in this section omit the boilerplate `t("…")` calls in
their snippets for brevity; in real code, no string literal that
reaches the DOM is hard-coded.
