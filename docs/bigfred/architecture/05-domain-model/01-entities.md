### 3a.1 Entities

```go
// pkgs/server/domain/user.go
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
// pkgs/server/domain/vehicle.go
type Vehicle struct {
    ID          uint
    DCCAddress  uint16    // unique – DCC is a global namespace on the track
    OwnerUserID uint      // must currently fall inside owner's DCC pool
    Name        string
    Type        string    // "loco", "wagon-with-sound", ...

    // Function inheritance (§3a.6, goal 16). Three states:
    //   (nil, nil)   – stand-alone, vehicle owns its VehicleFunction rows
    //   (T,   nil)   – LINKED to template T; function list is virtual (read from TemplateFunction)
    //   (T,   ts)    – DETACHED, copy-on-write applied at `ts`; vehicle owns its rows; T kept for lineage
    TemplateID          *uint
    FunctionsDetachedAt *time.Time

    CreatedAt   time.Time
    UpdatedAt   time.Time
}

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
// pkgs/server/domain/lease.go
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
// pkgs/server/domain/interlocking.go
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
// pkgs/server/domain/takeover.go
// Request issued by a signalman wanting driving authority over a driver's
// vehicle or train. The driver has 15 seconds to reject.
type TakeoverTarget string // "vehicle" | "train"
type TakeoverState  string // "pending" | "granted" | "rejected" | "cancelled" | "expired"

type TakeoverRequest struct {
    ID              uint
    SignalmanUserID uint
    DriverUserID    uint
    Target          TakeoverTarget
    TargetID        uint        // vehicle.id or train.id
    RequestedAt     time.Time
    DecisionAt      *time.Time
    AutoGrantAt     time.Time   // RequestedAt + 15s
    State           TakeoverState
}
```

```go
// pkgs/server/domain/radio.go
// Walkie-talkie messages between signalmen and drivers use a closed
// vocabulary so that translations and UI buttons stay deterministic.
type RadioPhrase string

const (
    RadioStoppedAtSignal   RadioPhrase = "STOPPED_AT_SIGNAL_READY_TO_ENTER"
    RadioEntryPermitted    RadioPhrase = "ENTRY_PERMITTED"
    RadioCancelRoute       RadioPhrase = "CANCEL_ROUTE"
    RadioRouteSet          RadioPhrase = "ROUTE_SET"
    RadioAck               RadioPhrase = "ACK"
    RadioStopImmediately   RadioPhrase = "STOP_IMMEDIATELY"
    RadioReadyToDepart     RadioPhrase = "READY_TO_DEPART"
    RadioDepartureCleared  RadioPhrase = "DEPARTURE_CLEARED"
)

type RadioMessage struct {
    ID              uint
    FromUserID      uint
    ToUserID        *uint // nil if directed at an interlocking
    ToInterlockingID *uint
    Phrase          RadioPhrase
    Note            string    // optional free-text, capped (e.g. 80 chars)
    SentAt          time.Time
}
```

```go
// pkgs/server/domain/layout.go
type LayoutConnectionType string

const (
    LayoutConnLoconetSerial LayoutConnectionType = "loconet_serial" // physical socket
    LayoutConnZ21           LayoutConnectionType = "z21"            // Z21 over network
    LayoutConnLoconetTCP    LayoutConnectionType = "loconet_tcp"    // LocoNet over Network
)

// Connection describes how the backend reaches the command station for
// this layout. Different connection types use different fields; the
// struct is intentionally flat so it serialises trivially to JSON in
// REST responses.
type LayoutConnection struct {
    Type     LayoutConnectionType
    Device   string // loconet_serial: e.g. "/dev/ttyUSB0"
    Baudrate int    // loconet_serial: e.g. 57600
    Address  string // z21 / loconet_tcp: host or IP
    Port     uint16 // z21 / loconet_tcp: TCP/UDP port
}

// Layout (Polish: makieta) – a physical model railway layout plus its
// command-station endpoint. Editable only by admin.
type Layout struct {
    ID         uint
    Name       string           // unique
    Connection LayoutConnection // stored as JSON column in SQLite
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

```go
// pkgs/server/domain/party.go
// Party (Polish: impreza) – a modeling event / room. No end date.
//
// LayoutID is nullable EXCLUSIVELY for the bootstrap `default` party:
//   - default party:    LayoutID = nil  → driver picks layout per session
//                                          via session.setLayout (§4.5.x)
//   - any other party:  LayoutID != nil → fixed layout for the whole party
//
// The "LayoutID nil iff name = 'default'" invariant is enforced both
// by PartyService and by a DB CHECK constraint. Joining a NON-default
// party whose layout has been deleted is impossible at the service
// layer (FK enforcement).
type Party struct {
    ID        uint
    Name      string    // unique; the system-seeded row is name = "default"
    LayoutID  *uint     // REQUIRED for non-default parties; nil only for `default`
    CreatedBy uint      // admin user that created it (0 for `default` seed)
    CreatedAt time.Time
    UpdatedAt time.Time

    Signalmen     []PartySignalman    `ref:"id" fk:"party_id"`
    Interlockings []PartyInterlocking `ref:"id" fk:"party_id"`
}

// IsDefault returns true for the bootstrap `default` party. The
// PartyService and the security policies use this helper rather than
// hard-coding the literal string.
func (p Party) IsDefault() bool { return p.Name == "default" }

// LayoutPickedPerSession returns true when this party requires the
// driver to pick a layout for each drive session (currently only the
// `default` party). Mirrors the storage invariant LayoutID == nil.
func (p Party) LayoutPickedPerSession() bool { return p.LayoutID == nil }

// PartySignalman grants the signalman role to UserID, but ONLY while
// they are active in PartyID. The grant is administered by an admin
// and may optionally carry an ExpiresAt (otherwise it is permanent
// inside the party). See §7a.2 for how this changes effective roles.
type PartySignalman struct {
    ID         uint
    PartyID    uint
    UserID     uint
    GrantedBy  uint      // admin user ID
    GrantedAt  time.Time
    ExpiresAt  *time.Time // nil = permanent inside this party
}

// PartyInterlocking whitelists which interlockings are visible to
// drivers (and which may be occupied) within a specific party. Both
// the admin and any signalman of the party may add rows; only admin
// may remove them.
type PartyInterlocking struct {
    ID              uint
    PartyID         uint
    InterlockingID  uint
    AddedByUserID   uint
    AddedAt         time.Time
}
```

```go
// pkgs/server/domain/audit.go
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

    AuditLayoutCreated AuditAction = "layout.created"
    AuditLayoutUpdated AuditAction = "layout.updated"
    AuditLayoutDeleted AuditAction = "layout.deleted"

    AuditPartyCreated AuditAction = "party.created"
    AuditPartyUpdated AuditAction = "party.updated"
    AuditPartyDeleted AuditAction = "party.deleted"

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
    ObjectType  string    // "vehicle" | "train" | "layout" | "party" | "session"
    ObjectID    uint      // ("object ID")
    ObjectName  string    // e.g. vehicle.name at write time ("object name")

    // Optional structured details for richer UIs. The audit log stays
    // readable without it; it is purely informational.
    PartyID  *uint  // where the action happened, if applicable
    Metadata string // JSON-encoded; e.g. for lease: {to_user_id, to_login, expires_at}
}
```

```go
// pkgs/server/domain/function.go
// FunctionIcon is a CLOSED catalogue. The frontend ships matching SVG
// assets; adding a new icon requires changing this enum AND adding the
// asset. Tygo (§ Tech stack) re-generates the TypeScript union.
type FunctionIcon string

const (
    IconHighHorn    FunctionIcon = "high_horn"
    IconLowHorn     FunctionIcon = "low_horn"
    IconHeadlight   FunctionIcon = "headlight"
    IconTaillight   FunctionIcon = "taillight"
    IconShunting    FunctionIcon = "shunting_mode"
    IconEngineStart FunctionIcon = "engine_start"
    IconBell        FunctionIcon = "bell"
    IconCabLight    FunctionIcon = "cab_light"
    IconCoupler     FunctionIcon = "coupler"
    IconSmoke       FunctionIcon = "smoke"
    IconDoorOpen    FunctionIcon = "door_open"
    IconBrake       FunctionIcon = "brake"
    IconSander      FunctionIcon = "sander"
    IconCompressor  FunctionIcon = "compressor"
    IconAnnounce    FunctionIcon = "announce"
    IconWhistle     FunctionIcon = "whistle"
    // ...catalogue is extended by editing this file + adding an SVG asset.
)

// FunctionKind distinguishes a press-and-hold horn from a latched light.
type FunctionKind string

const (
    FunctionLatched   FunctionKind = "latched"   // F0 lights stay on until toggled off
    FunctionMomentary FunctionKind = "momentary" // F2 horn active only while pressed
)

// VehicleFunction is one F0–F32 slot REGISTERED on a vehicle. The
// triplet (VehicleID, Num) is UNIQUE; Num is constrained 0..32.
//
// Rows exist only when the vehicle is STAND-ALONE or has been
// DETACHED from a template (§3a.6). For LINKED vehicles, the function
// list is read live from TemplateFunction.
type VehicleFunction struct {
    ID        uint
    VehicleID uint
    Num       uint8       // 0..32 inclusive
    Name      string
    Icon      FunctionIcon
    Kind      FunctionKind
    Position  int         // ordering inside the throttle UI grid
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

```go
// pkgs/server/domain/template.go
// VehicleTemplate – a reusable definition of a function list for a
// class of vehicles. Owner (or admin) may edit; any user may use a
// template to seed a new vehicle (goal 16).
type VehicleTemplate struct {
    ID          uint
    Name        string    // unique; user-facing
    Description string
    OwnerUserID uint
    Version     int       // monotonic; bumped on every mutation of either
                          // the template itself or any TemplateFunction it owns.
                          // Snapshots stored on Vehicle for diff detection.
    CreatedAt   time.Time
    UpdatedAt   time.Time

    Functions []TemplateFunction `ref:"id" fk:"template_id"`
}

// TemplateFunction mirrors VehicleFunction but at the template level.
// Tuple (TemplateID, Num) is UNIQUE; Num is constrained 0..32.
type TemplateFunction struct {
    ID         uint
    TemplateID uint
    Num        uint8
    Name       string
    Icon       FunctionIcon
    Kind       FunctionKind
    Position   int
}
```

```go
// pkgs/server/domain/script.go
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
//     alongside F0..F32 without a second icon set.
type Script struct {
    ID          uint
    OwnerUserID uint
    Name        string        // user-facing; unique per owner
    Description string
    Source      string        // JavaScript source code; size capped (64 KiB)
    Runtime     ScriptRuntime // ScriptRuntimeGoja
    Icon        FunctionIcon  // same closed catalogue as VehicleFunction.Icon
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
