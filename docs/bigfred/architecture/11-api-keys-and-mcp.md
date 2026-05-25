## 7b. API Keys and Built-in MCP Server

The same Go binary that serves REST + WebSocket also hosts a Model
Context Protocol (MCP) server, so that AI assistants, IDE agents and
automation scripts can drive the command station through Anthropic's
[Model Context Protocol](https://modelcontextprotocol.io/). The MCP
surface re-uses the same services (`LocoService`, `RadioService`, …) as
the human-facing UI – there is no second implementation of the business
rules.

### 7b.1 API key lifecycle

```
                ┌──────────────┐    POST /apikeys (≤ 365d)
                │ User in UI   │ ─────────────────────────────────────►
                └──────────────┘                                      │
                                                                      ▼
                                          ┌────────────────────────────────┐
                                          │ APIKeyService.Create:           │
                                          │   1. validate ExpiresAt-Now ≤   │
                                          │      365 days                   │
                                          │   2. generate "rb_" + 24 random │
                                          │      base64 chars               │
                                          │   3. hash secret part, store    │
                                          │      KeyPrefix + KeyHash        │
                                          │   4. return plaintext ONCE      │
                                          └─────────────┬───────────────────┘
                                                        ▼
                                              "rb_abcd1234.SECRET..."
                                              (shown one time in the UI;
                                               copy-to-clipboard + download)
```

Verification path (REST, MCP/HTTP, or stdio):

```
incoming request → header/query/env "rb_<prefix>.<secret>"
                 → APIKeyService.Verify:
                     - look up row by KeyPrefix (indexed)
                     - constant-time compare hash of <secret>
                     - reject if RevokedAt != nil or ExpiresAt < now()
                     - update LastUsedAt (async)
                 → returns auth.Identity { user, effectiveRoles, dccPool, scopes }
                 → same downstream middleware as login-based sessions
```

`APIKeyService.Create`:

```go
const APIKeyMaxLifetime = 365 * 24 * time.Hour

func (s *APIKeyService) Create(ctx context.Context, ownerID uint, name string,
    expiresAt time.Time, scopes []string) (plaintext string, key domain.APIKey, err error) {

    if expiresAt.Sub(time.Now()) > APIKeyMaxLifetime {
        return "", domain.APIKey{}, ErrAPIKeyLifetimeTooLong
    }
    // "rb_" + 12 chars prefix (public) + "." + 24 chars secret
    prefix := randomBase62(12)
    secret := randomBase62(24)
    plaintext = "rb_" + prefix + "." + secret

    key = domain.APIKey{
        UserID:    ownerID,
        Name:      name,
        KeyPrefix: prefix,
        KeyHash:   hashSecret(secret), // argon2id or sha256-hmac with server pepper
        Scopes:    strings.Join(scopes, ","),
        CreatedAt: time.Now(),
        ExpiresAt: expiresAt,
    }
    return plaintext, key, s.repo.Insert(ctx, &key)
}
```

### 7b.2 Authentication carriers

The same key works through three transports:

| Transport         | Where the key goes                                       |
|-------------------|----------------------------------------------------------|
| REST              | `Authorization: Bearer rb_<prefix>.<secret>`             |
| MCP over HTTP/SSE | Same `Authorization: Bearer …` header on the SSE upgrade |
| MCP over stdio    | Environment variable `BIGFRED_API_KEY` read by the client (e.g. Claude Desktop / Cursor) and forwarded by the spawned `loco server --mcp-stdio` process |

`APIKeyMiddleware` is just one more entry point into `auth.Identity`;
once an identity is attached to the request context, the existing
`RequireRole` / `RequireVehicleDrive` / `RequireVehicleEdit` middleware
keeps working unchanged – API keys are *not* a privilege escalation, they
are merely another way to authenticate as the owning user, restricted
further by the `Scopes` field.

### 7b.3 Exposed MCP tools

Mounted with [`github.com/mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go).
Each MCP tool is a thin wrapper around the same service method that the
REST / WS layer calls:

| MCP tool                  | Required scope         | Underlying call                                |
|---------------------------|------------------------|------------------------------------------------|
| `loco.list`               | `loco.read`            | `LocoService.List(identity)`                   |
| `loco.get`                | `loco.read`            | `LocoService.GetState(identity, addr)`         |
| `loco.set_speed`          | `loco.drive`           | `LocoService.SetSpeed(identity, addr, …)`     |
| `loco.toggle_fn`          | `loco.drive`           | `LocoService.ToggleFn(identity, addr, fn, on)` |
| `train.set_speed`         | `loco.drive`           | `LocoService.SetTrainSpeed(identity, …)`       |
| `radio.send`              | `radio.send`           | `RadioService.Send(identity, to, phrase, …)`   |
| `interlocking.list`       | `interlocking.read`    | `InterlockingService.List()`                   |
| `system.status`           | `system.read`          | `LocoService.SystemStatus()`                   |

The set is intentionally **smaller than the REST API** – CV writes,
admin endpoints (user/role/DCC pool management) and API-key minting
itself are deliberately not exposed via MCP, to keep blast radius of a
leaked key small.

### 7b.4 Mounting MCP next to chi

```go
// pkgs/server/mcp/server.go
package mcp

import (
    "context"

    "github.com/mark3labs/mcp-go/server"

    "github.com/keskad/loco/pkgs/server/service"
)

func New(loco *service.LocoService, radio *service.RadioService) *server.MCPServer {
    s := server.NewMCPServer("bigfred", "0.1.0",
        server.WithToolCapabilities(true),
    )
    registerLocoTools(s, loco)
    registerRadioTools(s, radio)
    return s
}
```

```go
// pkgs/server/mcp/tools_loco.go
import (
    "context"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

func registerLocoTools(s *server.MCPServer, loco *service.LocoService) {
    setSpeed := mcp.NewTool("loco.set_speed",
        mcp.WithDescription("Set the speed and direction of a locomotive"),
        mcp.WithNumber("addr",    mcp.Required(), mcp.Description("DCC address")),
        mcp.WithNumber("speed",   mcp.Required(), mcp.Description("0..127")),
        mcp.WithBoolean("forward",                mcp.Description("Travel direction; default true")),
    )

    s.AddTool(setSpeed, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        ident := authFromCtx(ctx) // injected by API-key middleware
        addr  := uint16(req.Params.Arguments["addr"].(float64))
        speed := uint8(req.Params.Arguments["speed"].(float64))
        fwd, _ := req.Params.Arguments["forward"].(bool)

        if err := loco.SetSpeed(ctx, ident, addr, speed, fwd); err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        return mcp.NewToolResultText("ok"), nil
    })
}
```

```go
// pkgs/server/main.go – mount both HTTP and MCP from a single process
sseHandler := server.NewSSEServer(mcpSrv,
    server.WithBaseURL("/mcp"),
)

r.Route("/mcp", func(r chi.Router) {
    r.Use(apikey.Middleware(apikeySvc)) // identity from "Authorization: Bearer rb_…"
    r.Use(apikey.RequireScope("loco.read")) // baseline scope to even connect
    r.Mount("/", sseHandler)
})

// Additionally, a "loco server --mcp-stdio" subcommand starts the MCP
// server on stdio for local tools like Claude Desktop / Cursor.
if mcpStdio {
    return server.ServeStdio(mcpSrv) // reads BIGFRED_API_KEY from env
}
```

### 7b.5 Why mount MCP inside the same binary

- **Single source of truth.** REST, WebSocket and MCP all go through the
  same service layer, the same authorization middleware and the same
  audit log. A bug fixed in `LocoService.SetSpeed` is fixed everywhere.
- **No extra deployment.** Hobby-grade hardware (a small home server)
  runs one process, not three.
- **Per-user, time-bound access.** Because MCP authenticates with the
  same per-user API keys (max 365 days, scoped, revocable), an AI agent
  acting "on behalf of" a driver only ever has that driver's
  permissions, and only for as long as the key lives.
