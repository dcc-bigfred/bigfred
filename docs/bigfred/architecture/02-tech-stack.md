## 1. Technology Stack

### Backend (Go)

| Layer            | Choice                                                | Rationale                                                                                          |
|------------------|-------------------------------------------------------|----------------------------------------------------------------------------------------------------|
| HTTP router      | **`github.com/go-chi/chi/v5`**                        | Lightweight, idiomatic, `net/http` compatible. Matches the existing style (stdlib + cobra/viper).  |
| WebSocket        | **`github.com/coder/websocket`** (ex `nhooyr/websocket`) | Context-aware, simpler API than `gorilla/websocket`, actively maintained.                       |
| Database         | **`modernc.org/sqlite`**                              | Already in use. Pure Go, keeps cross-compile and `CGO_ENABLED=0` working.                          |
| ORM              | **REL ([`github.com/go-rel/rel`](https://github.com/go-rel/rel))** with the SQLite3 adapter (`github.com/go-rel/sqlite3`) | **Data Mapper** style ORM (separates domain structs from persistence – fits the existing `LocoApp` controller layout), database-agnostic, supports transactions, eager loading, query composition and a `reltest` package for mocking the repository in unit tests. |
| Migrations       | **`github.com/go-rel/rel/cmd/rel`** (REL's own migration tool) | Migrations live in Go code, compile-time checked, embed-friendly. Used for the schema described in §3a. |
| Cache / Bus      | **`github.com/redis/go-redis/v9`**                    | Cache + Pub/Sub for cross-process fan-out of locomotive state events.                              |
| MCP server       | **`github.com/mark3labs/mcp-go`**                     | Idiomatic Go SDK for the Model Context Protocol. Exposes the same `LocoService` as MCP **tools** alongside the REST + WS APIs. Supports both stdio (local) and Streamable HTTP / SSE (remote) transports so it can be mounted under `/mcp` next to `chi`. |
| Embedded JS engine | **`github.com/dop251/goja`**                        | Pure-Go ECMAScript 5.1 engine (with most of ES6). Used by the **sibling `scripts-executor` process** to run user-authored throttle scripts in a sandbox. No cgo, keeps `CGO_ENABLED=0`. Each running script owns one `*goja.Runtime` on one goroutine (Goja VMs are not goroutine-safe). `vm.Interrupt(...)` provides a clean way for the executor to stop runaway scripts on deadline, user request or dead-man's switch. |
| Logger           | `logrus`                                              | Already in use.                                                                                    |
| Config           | `viper`                                               | Already in use.                                                                                    |
| CLI              | `cobra`                                               | Already in use; expose `loco server` and `loco worker` subcommands.                                |

### Frontend (React)

| Layer            | Choice                                | Rationale                                                                                  |
|------------------|---------------------------------------|--------------------------------------------------------------------------------------------|
| Bundler / dev    | **Vite + TypeScript**                 | Fast dev server, HMR, simple production builds.                                            |
| Server state     | **TanStack Query (React Query)**      | REST caching, retries, deduplication.                                                      |
| Client state     | **Zustand**                           | Tiny, easy to integrate with the WebSocket layer. Redux Toolkit would be overkill.         |
| WebSocket        | **`react-use-websocket`** or a custom `useSocket` hook | Reconnect, lifecycle, simple API.                                            |
| UI               | **Material UI (MUI v9, `@mui/material`)** | Production-ready React component library implementing Material Design. Comprehensive set of accessible components (sliders, buttons, app bar, drawer, dialogs), built-in theming, responsive breakpoints suitable for both mobile and desktop. See [MUI Getting Started](https://mui.com/material-ui/getting-started/). |
| Icons            | **`@mui/icons-material`**             | Official Material Symbols / Material Icons packaged as React components – matches the locomotive control surface (play, stop, lightbulb, horn, etc.). |
| Routing          | **React Router**                      | Standard.                                                                                  |
| Codegen          | **`tygo`** (Go → TypeScript types)    | Keep the WS protocol types in sync between Go and TS automatically.                        |
| Script editor    | **Monaco editor** (`@monaco-editor/react`) | Embedded VS-Code-style editor used on the Scripts page for JavaScript source editing (syntax highlighting, basic IntelliSense). The frontend never executes JS – it only edits and submits it to the backend. |
