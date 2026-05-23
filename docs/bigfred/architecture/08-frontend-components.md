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
    mode: "dark", // a layout console is easier to read in dark mode
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
