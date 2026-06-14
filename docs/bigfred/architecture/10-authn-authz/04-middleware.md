### 7a.4 Middleware – using the policies

The HTTP middleware (and the WebSocket dispatcher) is a **thin
adapter**: it loads the entities, hands them to the relevant
`*SecurityContext`, and translates `Decision.Reason` into an HTTP /
WebSocket error code.

```go
// pkgs/bigfred/server/http/middleware.go
//
// RequireRole consults the EFFECTIVE role set inside the JWT-pinned
// layout (§7a.2). Sudo admins pass the same gate as permanent admins
// — there is no "non-sudo" carve-out (§7a.7).
func RequireRole(auth *service.AuthService, roles ...domain.Role) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id, _ := IdentityFromContext(r.Context())
            eff, err := auth.Effective(r.Context(), id.User, id.Layout.ID)
            if err != nil {
                http.Error(w, "internal_error", http.StatusInternalServerError)
                return
            }
            for _, role := range roles {
                if eff.Has(role) {
                    next.ServeHTTP(w, r)
                    return
                }
            }
            http.Error(w, "forbidden", http.StatusForbidden)
        })
    }
}

// RequireVehicleDrive loads the loco, the active lease and the active
// takeover, then defers to LocoSecurityContext.CanDriveLoco.
func RequireVehicleDrive(repo VehicleAccessRepo) func(http.Handler) http.Handler {
    var sec security.LocoSecurityContext // stateless – zero value is fine
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id   := auth.FromCtx(r.Context())
            addr := uint16(chiParamInt(r, "addr"))
            loco, lease, takeover, err := repo.LoadDriveContext(r.Context(), addr)
            if err != nil {
                http.Error(w, "not found", http.StatusNotFound); return
            }
            d := sec.CanDriveLoco(security.LocoDriveInput{
                Actor: id.User, Loco: loco,
                ActiveLease: lease, ActiveTakeover: takeover,
                Now: time.Now(),
            })
            if !d.Allowed {
                http.Error(w, d.Reason, http.StatusForbidden); return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

The WebSocket dispatcher uses the same security context directly:

```go
func (c *Client) handle(ctx context.Context, env Envelope) {
    switch env.Type {
    case "loco.setSpeed":
        var p struct{ Addr uint16; Speed uint8; Forward bool }
        _ = json.Unmarshal(env.Payload, &p)

        loco, lease, takeover, err := c.repo.LoadDriveContext(ctx, p.Addr)
        if err != nil { c.ack(env.ID, err); return }

        d := c.sec.Loco.CanDriveLoco(security.LocoDriveInput{
            Actor: c.identity.User, Loco: loco,
            ActiveLease: lease, ActiveTakeover: takeover,
            Now: time.Now(),
        })
        if !d.Allowed {
            c.ack(env.ID, fmt.Errorf("forbidden: %s", d.Reason)); return
        }
        // …call LocoService.SetSpeed…
    }
}
```
