### 7a.3 Domain Policy Layer (`pkgs/server/security`)

All "is the actor allowed to do X to Y?" decisions live in **stateless
policy structs** under `pkgs/server/security`. The pattern has four
hard rules; together they keep authorization easy to reason about and
trivial to test.

**Rules of the security layer:**

1. **Stateless.** A policy struct has **no fields and no constructor
   dependencies** (no `*Repository`, no `*sql.DB`, no `context.Context`
   in its constructor). Methods are effectively pure functions.
2. **Takes domain objects, not IDs.** Every method accepts already-loaded
   `domain.*` entities and returns a `Decision`. Loading from the DB is
   the caller's responsibility (service or middleware) – the policy
   never reaches out.
3. **Pure domain language.** Inside the policy you may only refer to
   `pkgs/server/domain` types and `time.Time`. No HTTP, no SQL, no
   `errors.Is` against transport-level errors.
4. **One `Decision` type.** Methods return `security.Decision`, never
   `(bool, error)`. The reason is a machine-readable string so HTTP can
   pick a status code and the UI can localise it.

This is a **Policy / Specification pattern** applied per aggregate: one
`*SecurityContext` per domain area. The "context" suffix follows the
convention used in DDD codebases for a stateless evaluator of a
bounded context's authorization rules.

#### 7a.3.1 `Decision` type

```go
// pkgs/server/security/decision.go
package security

type Decision struct {
    Allowed bool
    Reason  string // machine-readable, e.g. "not_owner", "lease_expired"
}

var Allow = Decision{Allowed: true}

func Deny(reason string) Decision {
    return Decision{Allowed: false, Reason: reason}
}
```

#### 7a.3.2 `LocoSecurityContext` – the canonical example

```go
// pkgs/server/security/loco.go
package security

import (
    "time"

    "github.com/keskad/loco/pkgs/server/domain"
)

// LocoSecurityContext is a stateless policy. Construct it with a zero
// value: var sec security.LocoSecurityContext.
type LocoSecurityContext struct{}

// LocoDriveInput groups everything the policy needs to decide whether
// the actor may drive this loco RIGHT NOW. The caller (service /
// middleware) is responsible for loading these objects.
type LocoDriveInput struct {
    Actor          domain.User
    Loco           domain.Vehicle
    ActiveLease    *domain.VehicleLease    // nil if no active lease
    ActiveTakeover *domain.TakeoverRequest // nil if no active takeover
    Now            time.Time
}

// CanDriveLoco implements the rule:
//   - the owner can drive, unless there is an active takeover against them;
//   - the signalman that holds an active takeover for this loco can drive;
//   - the lessee can drive while the lease is active and not revoked.
func (LocoSecurityContext) CanDriveLoco(in LocoDriveInput) Decision {
    if in.Loco.OwnerUserID == in.Actor.ID {
        if t := in.ActiveTakeover; t != nil &&
            t.State == "granted" &&
            t.DriverUserID == in.Actor.ID {
            return Deny("active_takeover_by_signalman")
        }
        return Allow
    }

    if t := in.ActiveTakeover; t != nil &&
        t.State == "granted" &&
        t.SignalmanUserID == in.Actor.ID &&
        t.Target == "vehicle" && t.TargetID == in.Loco.ID {
        return Allow
    }

    if l := in.ActiveLease; l != nil &&
        l.ToUserID == in.Actor.ID &&
        l.RevokedAt == nil &&
        l.ExpiresAt.After(in.Now) {
        return Allow
    }

    return Deny("not_authorized_to_drive")
}

// CanEditLoco implements the rule: only the owner can edit metadata or
// write CVs. Lessees and signalmen are explicitly rejected; their
// driving authority does NOT escalate to edit rights.
func (LocoSecurityContext) CanEditLoco(actor domain.User, loco domain.Vehicle) Decision {
    if loco.OwnerUserID == actor.ID {
        return Allow
    }
    return Deny("only_owner_can_edit")
}

// CanRegisterLoco enforces that newly-registered vehicles fall inside
// the actor's DCC pool. The pool is passed in explicitly to keep the
// method pure.
func (LocoSecurityContext) CanRegisterLoco(actor domain.User, dccAddr uint16, pool []domain.DCCAddressRange) Decision {
    for _, r := range pool {
        if dccAddr >= r.FromAddr && dccAddr <= r.ToAddr {
            return Allow
        }
    }
    return Deny("dcc_address_outside_pool")
}
```

#### 7a.3.3 Other policy contexts (signatures)

```go
// pkgs/server/security/train.go
type TrainSecurityContext struct{ Loco LocoSecurityContext }

type TrainDriveInput struct {
    Actor          domain.User
    Train          domain.Train
    Members        []domain.Vehicle             // resolved member vehicles
    ActiveLease    *domain.TrainLease
    ActiveTakeover *domain.TakeoverRequest
    Now            time.Time
}

func (TrainSecurityContext) CanDriveTrain(TrainDriveInput) Decision
func (TrainSecurityContext) CanEditTrain(actor domain.User, t domain.Train) Decision

// CanDriveMember is the per-member gate used by TrainService.SetSpeed.
// The train-level CanDriveTrain decides whether the actor has the
// right to TOUCH the train at all (owner / lessee / signalman in
// takeover); CanDriveMember then re-runs the per-vehicle policy for
// each TrainMember so a member currently under signalman takeover or
// with an expired per-vehicle lease can be skipped without blocking
// the whole consist.
func (TrainSecurityContext) CanDriveMember(actor domain.User, t domain.Train, m domain.Vehicle, in LocoDriveInput) Decision

// pkgs/server/security/lease.go
type LeaseSecurityContext struct{}

func (LeaseSecurityContext) CanLeaseOutVehicle(actor domain.User, vehicle domain.Vehicle, expiresAt, now time.Time) Decision
func (LeaseSecurityContext) CanRevokeVehicleLease(actor domain.User, lease domain.VehicleLease) Decision
func (LeaseSecurityContext) CanLeaseOutTrain(actor domain.User, train domain.Train, expiresAt, now time.Time) Decision
func (LeaseSecurityContext) CanRevokeTrainLease(actor domain.User, lease domain.TrainLease) Decision

// pkgs/server/security/interlocking.go
type InterlockingSecurityContext struct{}

// CanOccupy now also verifies that the interlocking is whitelisted in
// the actor's active layout (LayoutInterlocking row exists). The caller
// provides that row (or nil) – the policy stays pure.
func (InterlockingSecurityContext) CanOccupy(actor domain.User, ilk domain.Interlocking, layoutILK *domain.LayoutInterlocking, current *domain.InterlockingSession, layoutSignalman *domain.LayoutSignalman) Decision
func (InterlockingSecurityContext) CanDisplace(actor domain.User, current *domain.InterlockingSession, layoutSignalman *domain.LayoutSignalman) Decision // same layout-scoped signalman grant; used when join carries force:true
func (InterlockingSecurityContext) CanRequestTakeover(actor domain.User, current *domain.InterlockingSession) Decision

// pkgs/server/security/command_station.go
type CommandStationSecurityContext struct{}

func (CommandStationSecurityContext) CanEditCommandStation(actor domain.User) Decision  // admin only
func (CommandStationSecurityContext) CanViewConnection(actor domain.User) Decision // admin only

// pkgs/server/security/layout.go
type LayoutSecurityContext struct{}

// CanLoginToLayout is the policy behind POST /api/v1/auth/login. The
// layout is picked on the login form (§7a.1); this method gates the
// transition from "valid credentials" to "session opened in layout".
// A locked layout is rejected so a hand-crafted request cannot bypass
// the filter applied by GET /api/v1/layouts/login.
func (LayoutSecurityContext) CanLoginToLayout(actor domain.User, p domain.Layout) Decision

// Layout-management policies. Every method here takes a
// `domain.EffectiveRoles` (§7a.2) instead of a bare `domain.User`
// because the rule for these operations is **non-sudo `admin` only**:
// a sudo-elevated `admin` is operationally allowed (§7a.7) but is
// explicitly denied edits to the layout itself, otherwise a sudo user
// could rotate the layout admin PIN to lock the real admin out. The
// shared rejection reason is `requires_non_sudo_admin`.
func (LayoutSecurityContext) CanCreateLayout(eff domain.EffectiveRoles) Decision                                  // non-sudo admin only
func (LayoutSecurityContext) CanEditLayout(eff domain.EffectiveRoles, p domain.Layout) Decision                   // non-sudo admin only; system layout name/isSystem still rejected via service-side rules
func (LayoutSecurityContext) CanDeleteLayout(eff domain.EffectiveRoles, p domain.Layout) Decision                 // non-sudo admin only; system layout (p.IsSystem) is undeletable

// CanRotateAdminPIN is the policy behind PUT /api/v1/layouts/{id}
// when the request body carries a non-empty `adminPin`. The rule is
// the same as CanEditLayout (non-sudo admin only) AND it is the
// single most important place this discrimination matters: a sudo
// admin must never be able to rotate the PIN that gated their own
// elevation, otherwise the layout's PIN can be silently changed
// during the 2-minute window. The system layout is allowed (the
// system layout still needs a rotatable PIN even though name /
// command-station-set are immutable).
func (LayoutSecurityContext) CanRotateAdminPIN(eff domain.EffectiveRoles, p domain.Layout) Decision               // non-sudo admin only

// Lock / unlock toggle Layout.Locked. The system layout cannot be
// locked (CanLockLayout returns Deny("default_layout_cannot_be_locked")
// when p.IsSystem == true). Unlocking a non-locked layout is a no-op
// at the service layer; the policy still gates it on non-sudo admin.
func (LayoutSecurityContext) CanLockLayout(eff domain.EffectiveRoles, p domain.Layout) Decision                   // non-sudo admin only; deny if p.IsSystem
func (LayoutSecurityContext) CanUnlockLayout(eff domain.EffectiveRoles, p domain.Layout) Decision                 // non-sudo admin only

// Attach / detach command stations to a non-system layout. The system
// layout exposes the full catalogue virtually, so both methods deny
// with "default_layout_command_stations_immutable" when p.IsSystem ==
// true. DetachCommandStation must additionally refuse to leave a
// non-system layout with zero stations
// ("layout_needs_at_least_one_command_station") – the caller supplies
// the current count.
func (LayoutSecurityContext) CanAttachCommandStation(eff domain.EffectiveRoles, p domain.Layout) Decision         // non-sudo admin only
func (LayoutSecurityContext) CanDetachCommandStation(eff domain.EffectiveRoles, p domain.Layout, currentCount int) Decision // non-sudo admin only

func (LayoutSecurityContext) CanAddSignalman(eff domain.EffectiveRoles, p domain.Layout) Decision                 // non-sudo admin only
func (LayoutSecurityContext) CanRemoveSignalman(eff domain.EffectiveRoles, p domain.Layout) Decision              // non-sudo admin only

// Adding an interlocking is allowed for non-sudo admin OR a signalman
// of THIS layout (sudo signalman is fine here – this is operational,
// not organisational). Caller passes the matching LayoutSignalman row
// (or nil); the policy resolves "is the actor a signalman here?" via
// `eff.Has(domain.RoleSignalman) && actorIsSignalmanHere != nil`.
func (LayoutSecurityContext) CanAddInterlocking(eff domain.EffectiveRoles, p domain.Layout, actorIsSignalmanHere *domain.LayoutSignalman) Decision
func (LayoutSecurityContext) CanRemoveInterlocking(eff domain.EffectiveRoles, p domain.Layout) Decision           // non-sudo admin only

// CanSudo gates `POST /api/v1/layouts/{id}/sudo` (§7a.7). Every
// authenticated user MAY call it from the layout they are logged into;
// the only structural rejection is a layout-id mismatch (caller's JWT
// `layoutId` must equal `p.ID`). PIN verification, rate-limiting and
// the sudo-row insert live in `AuthService.Sudo`, not in the policy
// layer (those are stateful concerns and the policy layer is pure).
func (LayoutSecurityContext) CanSudo(actor domain.User, p domain.Layout) Decision

// CanSetSessionCommandStation is the policy behind WS
// `session.setCommandStation`. The picked station must be currently
// attached to the session's layout: for non-system layouts the caller
// supplies the matching LayoutCommandStation row (or nil), for the
// system layout the caller supplies the live CommandStation row (or
// nil); a nil in either branch denies with
// "command_station_not_attached_to_layout". The actor only needs to
// be the session owner – every authenticated driver may pick.
func (LayoutSecurityContext) CanSetSessionCommandStation(actor domain.User, p domain.Layout, attachment *domain.LayoutCommandStation, catalogue *domain.CommandStation) Decision

// pkgs/server/security/audit.go
// The audit log is read-only and admin-only. There is no Can*Write
// policy because writes never originate from a user request – they
// originate from other services after a successful mutation.
type AuditSecurityContext struct{}

func (AuditSecurityContext) CanReadAuditLog(actor domain.User) Decision  // admin only

// pkgs/server/security/function.go
// Vehicle function DEFINITION editing is owner-only. Invoking a function
// at runtime is allowed for anyone with current driving authority and
// re-uses LocoSecurityContext.CanDriveLoco; CanInvokeFunction simply
// validates that the function number is registered on the vehicle.
type FunctionSecurityContext struct{}

func (FunctionSecurityContext) CanEditFunctions(actor domain.User, vehicle domain.Vehicle) Decision
func (FunctionSecurityContext) CanInvokeFunction(actor domain.User, vehicle domain.Vehicle, num uint8, registered []domain.VehicleFunction) Decision

// pkgs/server/security/template.go
// Vehicle templates: anyone can create; owner or admin can edit/delete.
// Using a template to seed a new vehicle is allowed for any user.
type TemplateSecurityContext struct{}

func (TemplateSecurityContext) CanCreateTemplate(actor domain.User) Decision               // any authenticated user
func (TemplateSecurityContext) CanEditTemplate(actor domain.User, t domain.VehicleTemplate) Decision   // owner OR admin
func (TemplateSecurityContext) CanDeleteTemplate(actor domain.User, t domain.VehicleTemplate) Decision // owner OR admin

// pkgs/server/security/radio.go
type RadioSecurityContext struct{}

func (RadioSecurityContext) CanSendTo(actor domain.User, toUser *domain.User, toIlk *domain.Interlocking) Decision

// pkgs/server/security/user.go
type UserSecurityContext struct{}

func (UserSecurityContext) CanManageUsers(actor domain.User) Decision
func (UserSecurityContext) CanGrantTemporaryRole(actor domain.User, target domain.User, role domain.Role, expiresAt, now time.Time) Decision

// pkgs/server/security/apikey.go
type APIKeySecurityContext struct{}

const APIKeyMaxLifetime = 365 * 24 * time.Hour

func (APIKeySecurityContext) CanMint(actor domain.User, expiresAt, now time.Time) Decision // enforces ≤365d
func (APIKeySecurityContext) CanRevoke(actor domain.User, key domain.APIKey) Decision      // owner OR admin
```

The whole permission matrix from §7a.4 is implemented in this directory
and **nowhere else** – there is exactly one place to look and exactly
one place to fix when a rule changes.

#### 7a.3.4 Testing the policy layer

Because the policies are pure functions over domain structs, tests need
no mocks, no fixtures and no I/O – plain table tests are enough:

```go
// pkgs/server/security/loco_test.go
func TestLocoSecurity_CanDriveLoco(t *testing.T) {
    now := time.Now()
    sec := security.LocoSecurityContext{}

    cases := map[string]struct {
        in   security.LocoDriveInput
        want bool
        why  string // expected Reason when denied
    }{
        "owner without takeover -> allow": {
            in: security.LocoDriveInput{
                Actor: domain.User{ID: 1},
                Loco:  domain.Vehicle{ID: 10, OwnerUserID: 1},
                Now:   now,
            },
            want: true,
        },
        "owner with takeover granted against them -> deny": {
            in: security.LocoDriveInput{
                Actor: domain.User{ID: 1},
                Loco:  domain.Vehicle{ID: 10, OwnerUserID: 1},
                ActiveTakeover: &domain.TakeoverRequest{
                    DriverUserID:    1,
                    SignalmanUserID: 2,
                    Target:          "vehicle", TargetID: 10,
                    State:           "granted",
                },
                Now: now,
            },
            want: false,
            why:  "active_takeover_by_signalman",
        },
        "lessee with active lease -> allow": {
            in: security.LocoDriveInput{
                Actor: domain.User{ID: 2},
                Loco:  domain.Vehicle{ID: 10, OwnerUserID: 1},
                ActiveLease: &domain.VehicleLease{
                    ToUserID:  2,
                    ExpiresAt: now.Add(1 * time.Hour),
                },
                Now: now,
            },
            want: true,
        },
        "lessee with expired lease -> deny": {
            in: security.LocoDriveInput{
                Actor: domain.User{ID: 2},
                Loco:  domain.Vehicle{ID: 10, OwnerUserID: 1},
                ActiveLease: &domain.VehicleLease{
                    ToUserID:  2,
                    ExpiresAt: now.Add(-1 * time.Minute),
                },
                Now: now,
            },
            want: false,
            why:  "not_authorized_to_drive",
        },
    }

    for name, tc := range cases {
        t.Run(name, func(t *testing.T) {
            got := sec.CanDriveLoco(tc.in)
            require.Equal(t, tc.want, got.Allowed, "decision")
            if !got.Allowed {
                require.Equal(t, tc.why, got.Reason, "reason")
            }
        })
    }
}
```

Notice: no database, no REL, no HTTP, no time freezing libraries. The
test exercises **the full rule** for the loco-drive policy without
booting anything.
