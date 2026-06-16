## 5. Backend Components

Layering under `pkgs/bigfred/server/` is defined in
[§3.1 Backend layer responsibilities](./04-repository-layout.md#31-backend-layer-responsibilities):
`http` / `ws` terminate transport and authentication; `service` owns
validation, permission checks via `security`, orchestration, and
repositories.

> **Pre-migration shape.** This whole section describes the **current**
> server, where `service` (`LocoService`, the poller, etc.) still hosts the
> use-case layer. Under the target directory-role convention
> ([§3.0 Directory roles](./04-repository-layout.md#30-directory-roles-layering-glossary))
> that action logic belongs in `cmd`, and `service` narrows to miscellaneous
> helper structs. Treat the `*Service` types below as *legacy to migrate
> into `cmd`*.

> §7e supersedes the DCC dispatch parts of this section. The
> `LocoService.SetSpeed` and background poller described in §5.4 and
> §5.5 still describe the **M1 baseline**, but in the §7e milestone
> they move into the sibling `dcc-bus` daemon (§16-dcc-bus). The
> `loco-server`-side `LocoService` shrinks to a thin
> `LocoServiceDriver` that delegates throttle writes to
> `DccBusService.PublishCommand` over Redis. See
> [§7e.6 Server integration](../16-dcc-bus/06-server-integration.md)
> for the post-§7e shape.

### 5.1 WebSocket Hub

`pkgs/bigfred/server/ws/hub.go` – central registry of connected clients with a
channel-based broadcaster.

```go
package ws

import (
    "context"
    "sync"
)

type Hub struct {
    mu         sync.RWMutex
    clients    map[*Client]struct{}
    subs       map[uint16]map[*Client]struct{} // addr -> clients
    register   chan *Client
    unregister chan *Client
    broadcast  chan Event
}

func NewHub() *Hub {
    return &Hub{
        clients:    make(map[*Client]struct{}),
        subs:       make(map[uint16]map[*Client]struct{}),
        register:   make(chan *Client, 16),
        unregister: make(chan *Client, 16),
        broadcast:  make(chan Event, 256),
    }
}

func (h *Hub) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case c := <-h.register:
            h.mu.Lock()
            h.clients[c] = struct{}{}
            h.mu.Unlock()
        case c := <-h.unregister:
            h.mu.Lock()
            delete(h.clients, c)
            for _, set := range h.subs {
                delete(set, c)
            }
            h.mu.Unlock()
            close(c.send)
        case ev := <-h.broadcast:
            h.dispatch(ev)
        }
    }
}

func (h *Hub) Subscribe(c *Client, addr uint16) {
    h.mu.Lock()
    defer h.mu.Unlock()
    if h.subs[addr] == nil {
        h.subs[addr] = make(map[*Client]struct{})
    }
    h.subs[addr][c] = struct{}{}
}

func (h *Hub) dispatch(ev Event) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    if ev.Addr != 0 {
        for c := range h.subs[ev.Addr] {
            select {
            case c.send <- ev:
            default: // drop slow client
            }
        }
        return
    }
    for c := range h.clients {
        select {
        case c.send <- ev:
        default:
        }
    }
}
```

### 5.2 Per-Connection Client

`pkgs/bigfred/server/ws/client.go`:

```go
package ws

import (
    "context"
    "encoding/json"
    "time"

    "github.com/coder/websocket"
)

type Client struct {
    conn *websocket.Conn
    hub  *Hub
    svc  LocoService
    send chan Event
}

type Envelope struct {
    Type    string          `json:"type"`
    ID      string          `json:"id,omitempty"`
    Payload json.RawMessage `json:"payload,omitempty"`
}

func (c *Client) readLoop(ctx context.Context) {
    defer func() { c.hub.unregister <- c }()
    for {
        _, data, err := c.conn.Read(ctx)
        if err != nil {
            return
        }
        var env Envelope
        if err := json.Unmarshal(data, &env); err != nil {
            continue
        }
        c.handle(ctx, env)
    }
}

func (c *Client) writeLoop(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second) // ping
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case ev, ok := <-c.send:
            if !ok {
                return
            }
            data, _ := json.Marshal(ev)
            wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
            err := c.conn.Write(wctx, websocket.MessageText, data)
            cancel()
            if err != nil {
                return
            }
        case <-ticker.C:
            _ = c.conn.Ping(ctx)
        }
    }
}
```

### 5.3 Action Dispatch → LocoApp / Station

`pkgs/bigfred/server/ws/handlers.go`:

```go
func (c *Client) handle(ctx context.Context, env Envelope) {
    switch env.Type {
    case "loco.subscribe":
        var p struct {
            Addr uint16 `json:"addr"`
        }
        _ = json.Unmarshal(env.Payload, &p)
        c.hub.Subscribe(c, p.Addr)

        // send a snapshot immediately, so the UI doesn't wait for the poller
        if st, err := c.svc.GetState(ctx, p.Addr); err == nil {
            c.send <- Event{Type: "loco.state", Addr: p.Addr, Payload: st}
        }

    case "loco.setSpeed":
        var p struct {
            Addr    uint16
            Speed   uint8
            Forward bool
        }
        _ = json.Unmarshal(env.Payload, &p)
        err := c.svc.SetSpeed(ctx, p.Addr, p.Speed, p.Forward)
        c.ack(env.ID, err)

        // optimistic broadcast; the poller will eventually correct it
        c.hub.broadcast <- Event{
            Type: "loco.state", Addr: p.Addr,
            Payload: LocoState{Addr: p.Addr, Speed: p.Speed, Forward: p.Forward},
        }

    case "loco.toggleFn":
        // analogously -> svc.ToggleFn
    case "system.estop":
        // svc.EStop
    case "ping":
        c.send <- Event{Type: "pong"}
    }
}
```

### 5.4 LocoService – Thin Wrapper Over the Existing `LocoApp` (M1 baseline)

> After §7e, the body of `SetSpeed` becomes a one-liner that publishes
> a `loco.setSpeed` command on `dcc-bus:cmd:<L>:<C>` (Redis pub/sub)
> via `DccBusService.PublishCommand`. The `Station` interface and
> the call into `pkgs/loco/commandstation` live inside the
> `dcc-bus` daemon. The snippet below is preserved as the M1 baseline
> – useful when running `loco-server` standalone without the
> daemon (e.g. integration tests, `--no-supervisor` dev mode).


```go
package service

type LocoService struct {
    App   *app.LocoApp // existing controller
    Cache *cache.Redis
    Bus   *bus.Bus // emits events that the Hub forwards
}

func (s *LocoService) SetSpeed(ctx context.Context, addr uint16, speed uint8, fwd bool) error {
    if err := s.App.Station.SetSpeed(commandstation.LocoAddr(addr), speed, fwd, 128); err != nil {
        return err
    }
    state := LocoState{Addr: addr, Speed: speed, Forward: fwd, UpdatedAt: time.Now()}
    _ = s.Cache.SetLocoState(ctx, state)
    s.Bus.Publish(bus.LocoStateChanged{State: state})
    return nil
}
```

### 5.5 Background Poller (M1 baseline; moves into `dcc-bus` in §7e)

A DCC track is shared state – another throttle or another app may also be
driving locomotives. The backend therefore periodically polls
`Station.GetSpeed` / `ListFunctions` for the addresses that any WS client
is currently subscribed to and publishes diffs to the bus:

```go
func (p *Poller) Run(ctx context.Context, every time.Duration) {
    t := time.NewTicker(every)
    defer t.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-t.C:
            for _, addr := range p.hub.SubscribedAddrs() {
                speed, fwd, err := p.station.GetSpeed(commandstation.LocoAddr(addr))
                if err != nil {
                    continue
                }
                // compare with cache; if changed -> Bus.Publish
            }
        }
    }
}
```

### 5.6 Redis – Concrete Roles

1. **State cache** of locomotives (`HSET loco:state addr "{json}"`), so a
   new WS client receives a snapshot on `loco.subscribe` without waiting
   for the poller.
2. **Pub/Sub** – when more than one backend instance is running (for
   example a separate worker for polling), all instances subscribe to a
   `loco.events` channel and forward to their own WS clients. Locally the
   in-process `bus.Bus` is enough on its own.
3. **Rate limiting / last-command memo** – e.g. ignore duplicate
   `setSpeed` calls within 50 ms.

### 5.7 Router (chi) – Wiring It All Together

```go
r := chi.NewRouter()
r.Use(middleware.RequestID, middleware.Logger, middleware.Recoverer)
r.Use(cors.Handler(cors.Options{
    AllowedOrigins: []string{"http://localhost:5173"},
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
}))

r.Route("/api/v1", func(r chi.Router) {
    r.Get("/locos", h.ListLocos)
    r.Post("/locos", h.CreateLoco)
    r.Get("/locos/{addr}", h.GetLoco)
    r.Put("/locos/{addr}", h.UpdateLoco)
    r.Get("/system/status", h.SystemStatus)

    r.HandleFunc("/ws", func(w http.ResponseWriter, req *http.Request) {
        ws.ServeWS(hub, svc, w, req)
    })
})

// In production, serve the built frontend from the same process
r.Handle("/*", http.FileServer(http.Dir("web/dist")))

srv := &http.Server{Addr: ":8080", Handler: r}
```

`ws.ServeWS`:

```go
func ServeWS(h *Hub, s LocoService, w http.ResponseWriter, r *http.Request) {
    conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
        InsecureSkipVerify: false, // in production set OriginPatterns explicitly
    })
    if err != nil {
        return
    }
    c := &Client{conn: conn, hub: h, svc: s, send: make(chan Event, 64)}
    h.register <- c
    ctx := r.Context()
    go c.writeLoop(ctx)
    c.readLoop(ctx) // blocks until disconnect
    conn.Close(websocket.StatusNormalClosure, "")
}
```
