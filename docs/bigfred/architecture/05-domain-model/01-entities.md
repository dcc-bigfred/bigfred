### 3a.1 Entities

```go
// pkgs/bigfred/server/domain/user.go
type Role string // "driver" | "signalman" | "admin"

type User struct {
    ID           uint
    Login        string    // unique
    PINHash      string    // bcrypt/argon2id over the PIN
    Role         Role      // primary, permanent role
    CreatedAt    time.Time
    UpdatedAt    time.Time

    TempRoles    []TemporaryRole `ref:"id" fk:"user_id"`
    DCCPool      []DCCAddressRange `ref:"id" fk:"user_id"`
}

// Admin can grant a role for a limited time. When ExpiresAt < now() the grant
// is ignored by AuthService; a cleanup job removes expired rows.
type TemporaryRole struct {
    ID         uint
    UserID     uint
    Role       Role
    GrantedBy  uint      // admin user ID
    GrantedAt  time.Time
    ExpiresAt  time.Time
}

// A contiguous DCC address range allocated to a user by the admin.
// Several rows per user are allowed (e.g. 100..199 and 3001..3010).
type DCCAddressRange struct {
    ID       uint
    UserID   uint
    FromAddr uint16 // inclusive
    ToAddr   uint16 // inclusive
}

// Temporary API key minted by a user for themselves. Plaintext value
// is shown to the user EXACTLY ONCE at creation time and never stored
// in the database. KeyHash holds an argon2id (or sha256-hmac) hash of
// the secret part. KeyPrefix is the public, human-readable prefix
// ("rb_abc12345…") used to look the row up quickly without scanning
// every hash.
type APIKey struct {
    ID          uint
    UserID      uint      // owner; the key inherits this user's roles & pool
    Name        string    // user-friendly label (e.g. "home assistant")
    KeyPrefix   string    // first 12 chars of the plaintext, indexed unique
    KeyHash     string    // hash of the rest of the plaintext
    Scopes      string    // CSV of scopes: "loco.read,loco.drive,radio.send"
    CreatedAt   time.Time
    ExpiresAt   time.Time // enforced: ExpiresAt - CreatedAt ≤ 365 days
    LastUsedAt  *time.Time
    RevokedAt   *time.Time
}
```

```go
// pkgs/bigfred/server/domain/vehicle.go

// VehicleKind is the closed catalogue of physical vehicle classes
// that show up on a modeling layout. The values steer UI icons and
// filtering ("show me only locomotives") but are NOT used for DCC
// addressing – every kind may carry an optional DCC address.
type VehicleKind string

const (
    VehicleKindLoco         VehicleKind = "loco"           // "Lokomotywa"
    VehicleKindEMU          VehicleKind = "emu"            // "EZT" – elektryczny/diesel zespół trakcyjny
    VehicleKindDrivingWagon VehicleKind = "driving_wagon"  // "Wagon sterowniczy"
    VehicleKindTrolley      VehicleKind = "trolley"        // "Drezyna"
    VehicleKindWagon        VehicleKind = "wagon"          // "Wagon" (pasywny)
)

type Vehicle struct {
    ID          uint
    // DCCAddress is OPTIONAL (pointer) per goal 4:
    //   *  non-nil – the vehicle is steerable, the address must fall
    //                inside the owner's DCC pool, and (DCCAddress,
    //                command_station) is unique on the track;
    //   *  nil    – the vehicle is a DUMMY: still listed in the
    //                catalogue, still attachable to a train, still
    //                visible on the layout roster, but the throttle
    //                never sends DCC against it. Typical for unpowered
    //                wagons and visual fillers.
    DCCAddress  *uint16
    OwnerUserID uint      // when DCCAddress is set, the address must lie inside the owner's DCC pool
    Name        string
    Kind        VehicleKind // closed catalogue (loco | emu | driving_wagon | trolley | wagon)
    Number      string      // optional free-text inventory / road number (e.g. "ET22-123", "92510")

    // Function inheritance (§3a.6, goal 16). Three states:
    //   (nil, nil)   – stand-alone, vehicle owns rows in `dcc_functions` (vehicle_id set)
    //   (T,   nil)   – LINKED to template T; list is virtual (read `dcc_functions` WHERE template_id = T)
    //   (T,   ts)    – DETACHED, copy-on-write at `ts`; vehicle owns its rows; T kept for lineage
    TemplateID          *uint
    FunctionsDetachedAt *time.Time

    // Per-vehicle dead-man's switch catalogue (§7e.5). Function
    // numbers are F0..F31. Defaults: Rp1=F2, emergency lights=F0,
    // option=stop.
    Rp1Function             uint8               // horn / Rp1 output
    EmergencyLightsFunction uint8               // Pc6 / emergency lights
    DeadManSwitchOption     DeadManSwitchOption // stop | stop_horn | stop_horn_emergency_lights

    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type DeadManSwitchOption string

const (
    DeadManSwitchStop                    DeadManSwitchOption = "stop"
    DeadManSwitchStopHorn                DeadManSwitchOption = "stop_horn"
    DeadManSwitchStopHornEmergencyLights DeadManSwitchOption = "stop_horn_emergency_lights"
)

// Train (Polish: skład) – an ordered group of 1+ Vehicles addressed and
// driven as a single unit. See the Terminology table.
type Train struct {
    ID          uint
    OwnerUserID uint
    Name        string
    CreatedAt   time.Time
    UpdatedAt   time.Time

    Members     []TrainMember `ref:"id" fk:"train_id"`
}

type TrainMember struct {
    ID         uint
    TrainID    uint
    VehicleID  uint
    Position   int  // ordering inside the train
    Reversed   bool // vehicle coupled the other way around
}
```

```go
// pkgs/bigfred/server/domain/lease.go
// A vehicle or train can be leased to another user for DRIVING ONLY.
// Edit rights (CV writes, rename, delete, change train composition)
// always stay with the owner.
type VehicleLease struct {
    ID         uint
    VehicleID  uint
    FromUserID uint // owner
    ToUserID   uint // lessee
    StartedAt  time.Time
    ExpiresAt  time.Time
    RevokedAt  *time.Time // nil = active
}

type TrainLease struct {
    ID         uint
    TrainID    uint
    FromUserID uint
    ToUserID   uint
    StartedAt  time.Time
    ExpiresAt  time.Time
    RevokedAt  *time.Time
}
```

```go
// pkgs/bigfred/server/domain/interlocking.go
// A signal box / interlocking. At most one active session per interlocking.
type Interlocking struct {
    ID        uint
    Name      string
    Location  string // free-text description
    CreatedAt time.Time
}

// Enforced by a unique index: UNIQUE(interlocking_id) WHERE ended_at IS NULL.
type InterlockingSession struct {
    ID              uint
    InterlockingID  uint
    SignalmanUserID uint
    StartedAt       time.Time
    EndedAt         *time.Time
}
```

```go
// pkgs/bigfred/server/domain/takeover.go
// Request issued by a signalman wanting driving authority over a driver's
// vehicle or train. The driver has 15 seconds to reject; if they do not,
// the request is granted automatically (§4.3).
type TakeoverTarget string // "vehicle" | "train"
type TakeoverState  string // "pending" | "granted" | "rejected" | "cancelled" | "expired" | "released"

// TakeoverWindow is the driver's reject window; TakeoverLeaseDuration is
// how long the signalman holds the target once granted (the self-lease).
const (
	TakeoverWindow        = 15 * time.Second
	TakeoverLeaseDuration = 5 * time.Minute
)

type TakeoverRequest struct {
	ID              uint
	SignalmanUserID uint
	DriverUserID    uint
	Target          TakeoverTarget
	TargetID        uint        // vehicle.id or train.id
	RequestedAt     time.Time
	DecisionAt      *time.Time
	AutoGrantAt     time.Time   // RequestedAt + TakeoverWindow

	// On grant, the service creates a VehicleLease / TrainLease
	// (FromUserID = owner, ToUserID = signalman, ExpiresAt = grant +
	// TakeoverLeaseDuration) and stores its id here so release can revoke
	// it. The driver's throttle session for the target is ended and the
	// target hidden from their throttle picker until release (§4.3).
	GrantedLeaseID  *uint
	ReleasedAt      *time.Time
	State           TakeoverState
}
```

```go
// pkgs/bigfred/server/domain/radio.go
// Walkie-talkie messages between signalmen and drivers use a closed
// vocabulary so that translations and UI buttons stay deterministic.
//
// IMPORTANT: RadioMessage is NOT a SQLite-persisted entity. Radio is
// operational chatter, not an audit trail: messages live ONLY in Redis
// with a default 4-hour TTL and are gone afterwards (§4.4.4). This type
// is therefore a value object serialised into Redis streams, never a
// REL-mapped table row. (Radio Stop, by contrast, IS audited — see
// AuditSessionEmergencyExecuted / `system.radio_stop`.)
type RadioPhrase string

const (
	RadioStoppedAtSignal   RadioPhrase = "STOPPED_AT_SIGNAL_READY_TO_ENTER"
	RadioEntryPermitted    RadioPhrase = "ENTRY_PERMITTED"
	RadioCancelRoute       RadioPhrase = "CANCEL_ROUTE"
	RadioRouteSet          RadioPhrase = "ROUTE_SET"
	RadioAck               RadioPhrase = "ACK"
	RadioStopImmediately   RadioPhrase = "STOP_IMMEDIATELY" // walkie-talkie phrase only; see §4.6 for layout-wide Radio Stop
	RadioReadyToDepart     RadioPhrase = "READY_TO_DEPART"
	RadioDepartureCleared  RadioPhrase = "DEPARTURE_CLEARED"
)

// RadioMessage is a single walkie-talkie message. Stored ONLY in Redis
// (default TTL 4h, §4.4.4); ID is a Redis stream / ULID id, not a SQL
// primary key.
//
// Target  — exactly one of ToUserID / ToInterlockingID is set.
// Context — exactly one of ContextVehicleID / ContextTrainID is set; a
//           message is always ABOUT a specific vehicle or train so the
//           chat line can render "({fromLogin}) {context name}: {phrase}"
//           and the signalman can correlate it with the roster panel
//           (§4.4.1, §6.3d).
type RadioMessage struct {
	ID               string // Redis stream id / ULID (NOT a SQL id)
	LayoutID         uint
	FromUserID       uint
	FromLogin        string // denormalized for chat rendering & replay
	ToUserID         *uint  // nil if directed at an interlocking
	ToInterlockingID *uint  // nil if directed at a user

	// Context: exactly one of the two is non-nil (vehicle XOR train).
	ContextVehicleID *uint
	ContextTrainID   *uint
	ContextName      string // denormalized vehicle/train name for the chat line

	Phrase RadioPhrase
	Note   string // optional free-text, capped (e.g. 80 chars)
	SentAt time.Time
}
```

```go
// pkgs/bigfred/server/domain/command_station.go
type CommandStationConnectionType string

const (
    CommandStationConnLoconetSerial CommandStationConnectionType = "loconet_serial" // physical socket
    CommandStationConnZ21           CommandStationConnectionType = "z21"            // Z21 over network
    CommandStationConnLoconetTCP    CommandStationConnectionType = "loconet_tcp"    // LocoNet over Network
)

// Connection describes how the backend reaches the command station for
// this command station. Different connection types use different fields; the
// struct is intentionally flat so it serialises trivially to JSON in
// REST responses.
type CommandStationConnection struct {
    Type     CommandStationConnectionType
    Device   string // loconet_serial: e.g. "/dev/ttyUSB0"
    Baudrate int    // loconet_serial: e.g. 57600
    Address  string // z21 / loconet_tcp: host or IP
    Port     uint16 // z21 / loconet_tcp: TCP/UDP port
}

// CommandStation (Polish: centralka) – a physical model railway command station plus its
// command-station endpoint. Editable only by admin.
type CommandStation struct {
    ID         uint
    Name       string           // unique
    Connection CommandStationConnection // stored as JSON column in SQLite
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

```go
// pkgs/bigfred/server/domain/layout.go
// Layout (Polish: makieta) – a modeling event / room. The user picks a
// layout on the login form (§7a.1) and the resulting drive session is
// pinned to it for its entire lifetime.
//
// Two flags steer the lifecycle of a Layout row:
//
//   - IsSystem: true for the bootstrap row only. The system layout
//     cannot be deleted, cannot be locked, its Name and IsSystem fields
//     are immutable, and its set of attached command stations is a
//     virtual view of `command_stations` (admin endpoints that try to
//     mutate it return 422). The system row is seeded with
//     Name = "default"; the UI renders it via the i18n key
//     `layout:system_default_label` ("Domyślna (warsztat)" / "Default
//     (workshop)").
//
//   - Locked: false for every layout right after creation; an admin
//     may toggle it on a non-system layout via POST/DELETE on
//     /api/v1/layouts/{id}/lock. A locked layout is hidden from the
//     unauthenticated login dropdown (`GET /api/v1/layouts/login`) so no
//     new sessions can open in it; existing drive sessions in that
//     layout keep running until they close on their own. The system
//     layout cannot be locked (DB CHECK + service rule).
//
// A Layout has **one or more attached command stations** (see
// LayoutCommandStation below). The driver picks one of those at the
// throttle via `session.setCommandStation` (§4.5). There is no longer
// a single nullable CommandStationID column on Layout itself.
type Layout struct {
    ID        uint
    Name      string    // unique; system row is Name = "default" (immutable)
    IsSystem  bool      // true ONLY for the system-seeded row; immutable
    Locked    bool      // admin-toggleable on non-system layouts; always false for IsSystem rows

    // AdminPINHash is the argon2id-hashed **layout admin PIN** that
    // gates the sudo elevation flow (§7a.7). Plaintext PINs never
    // leave the request handler – the field is set on Create, rotated
    // by `PUT /api/v1/layouts/{id} { adminPin }` (admin-only;
    // sudo-elevated admins are explicitly forbidden) and persisted as
    // a hash with a per-row salt. The system layout's row is seeded
    // with a one-shot random PIN at bootstrap time and printed once
    // to the server log – an admin is expected to rotate it
    // immediately on first login. The column is **NEVER NULL**: no
    // layout can exist without a PIN, otherwise the sudo flow would
    // be unreachable. See §7a.7 for the lifecycle.
    AdminPINHash string

    CreatedBy uint      // admin user that created it (0 for the system seed)
    CreatedAt time.Time
    UpdatedAt time.Time

    Signalmen       []LayoutSignalman       `ref:"id" fk:"layout_id"`
    Interlockings   []LayoutInterlocking    `ref:"id" fk:"layout_id"`
    Vehicles        []LayoutVehicle         `ref:"id" fk:"layout_id"`
    CommandStations []LayoutCommandStation  `ref:"id" fk:"layout_id"` // EMPTY for IsSystem rows: their set is virtual
}

// IsDefault returns true for the bootstrap system layout. Callers must
// use this helper (or the IsSystem field) – never compare Name against
// the string literal, because the displayed name comes from i18n while
// the stored Name is a stable system marker.
func (p Layout) IsDefault() bool { return p.IsSystem }

// LayoutCommandStation pins a CommandStation to a non-system layout.
// Rows exist ONLY for layouts with IsSystem == false:
//
//   - the system layout's "set of command stations" is virtual: any
//     CommandStation row in the catalogue is implicitly attached,
//     including ones added after the system layout was seeded.
//   - inserting a row for a system layout is rejected with
//     `default_layout_command_stations_immutable` (DB CHECK on
//     `layout_id != <system_layout_id>` + service validation).
//
// Admin is the only writer; both adding and removing are audited
// (`layout.command_station_attached` / `layout.command_station_detached`).
// Deleting a CommandStation cascades: every LayoutCommandStation row
// pointing at it disappears, and any drive session currently pinned to
// that command station is gracefully detached (CommandStationID → nil;
// throttle re-gated until the user re-picks). See §3a.3 invariants.
type LayoutCommandStation struct {
    ID               uint
    LayoutID         uint
    CommandStationID uint
    AddedByUserID    uint      // admin user ID
    AddedAt          time.Time
}

// LayoutSignalman grants the signalman role to UserID, but ONLY while
// they are active in LayoutID. The grant is administered by an admin
// and may optionally carry an ExpiresAt (otherwise it is permanent
// inside the layout). See §7a.2 for how this changes effective roles.
type LayoutSignalman struct {
    ID         uint
    LayoutID    uint
    UserID     uint
    GrantedBy  uint      // admin user ID
    GrantedAt  time.Time
    ExpiresAt  *time.Time // nil = permanent inside this layout
}

// LayoutInterlocking whitelists which interlockings are visible to
// drivers (and which may be occupied) within a specific layout. Both
// the admin and any signalman of the layout may add rows; only admin
// may remove them.
type LayoutInterlocking struct {
    ID              uint
    LayoutID         uint
    InterlockingID  uint
    AddedByUserID   uint
    AddedAt         time.Time
}

// LayoutVehicle pins a registered Vehicle to a layout's operating roster.
// A vehicle must be registered globally before it can be added; only the
// vehicle owner may add or remove their row. The dashboard lists these
// rows so every participant in the layout sees which locos are "on the
// floor" for this session. Distinct from leasing: roster membership
// is visibility/participation, not a transfer of driving authority.
type LayoutVehicle struct {
    ID         uint
    LayoutID   uint
    VehicleID  uint
    AddedByUserID uint // must equal vehicle.OwnerUserID at insert time
    AddedAt    time.Time
}
```

```go
// pkgs/bigfred/server/domain/sudo.go

// EffectiveRoles is the flat result of
// `AuthService.Effective(ctx, user, layoutID)`. Permanent role,
// admin-issued temporary grants, layout-scoped signalman grants
// (admin-issued OR self-granted via the engineer's-cap icon) and
// the sudo admin elevation collapse onto the same set. A sudo
// admin grants the SAME authority as a permanent admin everywhere
// (§7a.7).
type EffectiveRoles struct {
    // unexported set; constructed by NewEffectiveRoles.
}

func NewEffectiveRoles(roles ...Role) EffectiveRoles
func (EffectiveRoles) Has(Role) bool

// SudoElevation is the short-lived, layout-scoped admin self-grant
// produced by the sudo flow (§7a.7). The signalman icon next to the
// padlock writes a permanent `LayoutSignalman` row instead, so this
// type is admin-only.
//
// Invariants (DB + service):
//   - exactly one ACTIVE row per (UserID, LayoutID), enforced by a
//     UNIQUE index. The "renew the timer" path is a single upsert
//     that pushes ExpiresAt forward;
//   - ExpiresAt - GrantedAt MUST equal the configured sudo TTL
//     (default 2 minutes; bounds [1m, 10m] enforced by the server
//     config – not by a per-row CHECK, because an operator may tune
//     the TTL between deployments and existing rows must remain
//     valid);
//   - the row is created ONLY by `SudoService.Sudo` after a
//     successful PIN verification against `Layout.AdminPINHash`.
//     There is no admin-side "grant sudo to user X" path – sudo is
//     always a self-grant.
type SudoElevation struct {
    ID        uint
    UserID    uint
    LayoutID  uint
    GrantedAt time.Time
    ExpiresAt time.Time
}
```

```go
// pkgs/bigfred/server/domain/audit.go
// AuditAction is a closed vocabulary of audit event types. Adding a new
// audited event requires adding it here AND wiring AuditService.Log in
// the matching service. Keeping the vocabulary closed makes the audit
// surface trivially diff-reviewable.
type AuditAction string

const (
    AuditVehicleCreated      AuditAction = "vehicle.created"
    AuditVehicleUpdated      AuditAction = "vehicle.updated"
    AuditVehicleDeleted      AuditAction = "vehicle.deleted"
    AuditVehicleLeased       AuditAction = "vehicle.leased"
    AuditVehicleLeaseRevoked AuditAction = "vehicle.lease_revoked"
    AuditVehicleLeaseExpired AuditAction = "vehicle.lease_expired"

    AuditTrainCreated      AuditAction = "train.created"
    AuditTrainUpdated      AuditAction = "train.updated"
    AuditTrainDeleted      AuditAction = "train.deleted"
    AuditTrainLeased       AuditAction = "train.leased"
    AuditTrainLeaseRevoked AuditAction = "train.lease_revoked"
    AuditTrainLeaseExpired AuditAction = "train.lease_expired"

    AuditCommandStationCreated AuditAction = "command_station.created"
    AuditCommandStationUpdated AuditAction = "command_station.updated"
    AuditCommandStationDeleted AuditAction = "command_station.deleted"

    AuditLayoutCreated                  AuditAction = "layout.created"
    AuditLayoutUpdated                  AuditAction = "layout.updated"
    AuditLayoutDeleted                  AuditAction = "layout.deleted"
    AuditLayoutLocked                   AuditAction = "layout.locked"
    AuditLayoutUnlocked                 AuditAction = "layout.unlocked"
    AuditLayoutCommandStationAttached   AuditAction = "layout.command_station_attached"
    AuditLayoutCommandStationDetached   AuditAction = "layout.command_station_detached"
    // Layout admin PIN was rotated through the layout settings
    // page. Any effective admin (permanent or sudo) may rotate it.
    // Metadata = {previous_hash_prefix} so the audit row never
    // carries plaintext or full hash. See §7a.7.
    AuditLayoutAdminPINChanged          AuditAction = "layout.admin_pin_changed"

    // Sudo elevation lifecycle (§7a.7). The actor is the user who
    // typed the layout PIN; ObjectType = "layout", ObjectID =
    // LayoutID, ObjectName = layout.Name at the time of the event.
    // Metadata for `auth.sudo_granted` and `auth.sudo_expired` is
    // `{ target, expiresAt }`; for `auth.sudo_revoked` it carries
    // `{ target, reason: "user_action"|"logout"|"layout_deleted" }`.
    AuditAuthSudoGranted AuditAction = "auth.sudo_granted"
    AuditAuthSudoRevoked AuditAction = "auth.sudo_revoked"
    AuditAuthSudoExpired AuditAction = "auth.sudo_expired"
    // A failed PIN attempt that triggered the rate-limit soft lock.
    // ObjectType = "layout"; Metadata = { target, attempts, lockedUntil }.
    AuditAuthSudoLocked  AuditAction = "auth.sudo_locked"

    // Vehicle function definitions (registration / detach / re-attach).
    // Runtime invocation (DCC F<n> ON/OFF) is NOT audited.
    AuditVehicleFunctionsUpdated  AuditAction = "vehicle.functions_updated"
    AuditVehicleFunctionsDetached AuditAction = "vehicle.functions_detached"
    AuditVehicleFunctionsAttached AuditAction = "vehicle.functions_attached"

    AuditTemplateCreated AuditAction = "template.created"
    AuditTemplateUpdated AuditAction = "template.updated"
    AuditTemplateDeleted AuditAction = "template.deleted"

    // Scripts (§3a.7). The audit row stores metadata only; the
    // JavaScript source body is NEVER copied into Metadata so that
    // deleting a script truly removes its source from the system.
    AuditScriptCreated  AuditAction = "script.created"
    AuditScriptUpdated  AuditAction = "script.updated"
    AuditScriptDeleted  AuditAction = "script.deleted"
    AuditScriptAttached AuditAction = "script.attached"
    AuditScriptDetached AuditAction = "script.detached"

    // "Driver fell asleep" – the dead-man's switch fired and the user's
    // emergency plan was executed (§4.5).
    AuditSessionEmergencyExecuted AuditAction = "session.emergency_executed"
)

// AuditLogEntry is the canonical row of the audit log. All six fields
// the spec requires (§ goal 14) are first-class. Object name and actor
// login are DENORMALIZED at write time so that later renames or
// deletions cannot rewrite history.
type AuditLogEntry struct {
    ID          uint
    Action      AuditAction
    ActorUserID uint      // the user that triggered the action ("user ID")
    ActorLogin  string    // user.login at the moment of the event ("user name")
    OccurredAt  time.Time // UTC, ms precision ("date")
    ObjectType  string    // "vehicle" | "train" | "command_station" | "layout" | "session"
    ObjectID    uint      // ("object ID")
    ObjectName  string    // e.g. vehicle.name at write time ("object name")

    // Optional structured details for richer UIs. The audit log stays
    // readable without it; it is purely informational.
    LayoutID  *uint  // where the action happened, if applicable
    Metadata string // JSON-encoded; e.g. for lease: {to_user_id, to_login, expires_at}
}
```

```go
// pkgs/bigfred/server/domain/function.go
// FunctionIcon is a CLOSED catalogue (67 values). The authoritative slug
// list with Polish labels lives in §3a.8 (08-function-icon-catalogue.md).
// The frontend ships one SVG per slug; Tygo re-generates the TS union.
type FunctionIcon string

const (
    IconUnspecified FunctionIcon = "unspecified"
    IconLight       FunctionIcon = "light"
    IconEngine      FunctionIcon = "engine"
  // …remaining members mirror §3a.8 exactly…
)

// DccFunction is one F0–F31 slot stored in the unified `dcc_functions`
// table. Exactly one of VehicleID or TemplateID is non-nil on every row
// (enforced by DB CHECK). Num is constrained 0..31.
//
// Template rows:  TemplateID != nil, VehicleID == nil
// Vehicle rows:   VehicleID != nil, TemplateID == nil
//
// For a LINKED vehicle (§3a.6) there are no vehicle rows yet; the
// effective list is read from rows WHERE template_id = vehicle.TemplateID.
// After detach, vehicle rows exist and template rows are unchanged.
type DccFunction struct {
    ID         uint
    VehicleID  *uint       // set on vehicle-owned rows; NULL on template rows
    TemplateID *uint       // set on template-owned rows; NULL on vehicle rows
    Num        uint8       // 0..31 inclusive
    Name       string
    Icon       FunctionIcon
    Position   int         // ordering inside the throttle UI grid
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

```go
// pkgs/bigfred/server/domain/template.go
// VehicleTemplate – a reusable definition of a function list for a
// class of vehicles. Owner (or admin) may edit; any user may use a
// template to seed a new vehicle (goal 16).
type VehicleTemplate struct {
    ID          uint
    Name        string    // unique; user-facing
    Description string
    OwnerUserID uint
    Version     int       // monotonic; bumped on every mutation of either
                          // the template itself or any DccFunction row with
                          // template_id = this template. Snapshots stored on
                          // Vehicle for diff detection.
    CreatedAt   time.Time
    UpdatedAt   time.Time

    Functions []DccFunction `ref:"template_id" fk:"template_id"` // rows WHERE template_id = id
}
```

```go
// pkgs/bigfred/server/domain/script.go
// ScriptRuntime names the embedded interpreter used to execute the
// script source. Today only Goja (pure-Go ECMAScript 5.1+) is wired
// up. The enum is kept open so future runtimes (e.g. a sandboxed
// Lua) can be added without an `omitempty`-style data migration.
type ScriptRuntime string

const ScriptRuntimeGoja ScriptRuntime = "goja" // github.com/dop251/goja

// Script – a piece of JavaScript source authored by a user and
// executed SERVER-SIDE inside a sandboxed Goja VM in the sibling
// scripts-executor process. Stored as plain text; the embedded
// runtime calls back through the server's services for every DSL
// operation (findFirstLoco, findByDCCAddr, setSpeed, funcOn/Off,
// sleep, …), so every action is authorized exactly like a manual
// throttle press.
//
// Ownership and edit rules:
//   - OwnerUserID is the only user who can edit Source / Name / Icon
//     / Runtime. The owner may, however, lease a vehicle that has
//     this script attached – the lessee will see and may RUN the
//     script but cannot view or modify its source.
//   - Icon is reused from the function-icon catalogue (FunctionIcon)
//     so the throttle UI can render scripts as additional buttons
//     alongside F0..F31 without a second icon set.
type Script struct {
    ID          uint
    OwnerUserID uint
    Name        string        // user-facing; unique per owner
    Description string
    Source      string        // JavaScript source code; size capped (64 KiB)
    Runtime     ScriptRuntime // ScriptRuntimeGoja
    Icon        FunctionIcon  // same closed catalogue as DccFunction.Icon
    Version     int           // monotonic; bumped on every Source/metadata edit.
                              // Currently only used to invalidate the editor's
                              // optimistic cache; server-side execution always
                              // loads the latest source at run.start time.
    DeadlineSec int           // hard wall-clock cap for a single run (default 60,
                              // max 600). After this time the executor calls
                              // vm.Interrupt("timeout") regardless of state.
    CreatedAt time.Time
    UpdatedAt time.Time

    Attachments []ScriptAttachment `ref:"id" fk:"script_id"`
}

// ScriptAttachment binds a Script to exactly one Vehicle XOR one
// Train. The attachment, not the Script itself, carries the
// per-throttle metadata (position on the button row).
//
// Invariants enforced by service + DB:
//   - exactly one of VehicleID / TrainID is set (CHECK constraint);
//   - a Script may be attached MULTIPLE times (e.g. the same "yard
//     shunt" script can be wired to several locos), but a given
//     (Script, Vehicle) or (Script, Train) pair is UNIQUE so the
//     button does not show up twice on one throttle.
type ScriptAttachment struct {
    ID        uint
    ScriptID  uint
    VehicleID *uint     // exactly one of VehicleID / TrainID is set
    TrainID   *uint
    Position  int       // sort order in the throttle UI
    CreatedAt time.Time
}
```
