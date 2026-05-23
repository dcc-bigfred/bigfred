### 4.1 REST

All endpoints live under `/api/v1`. Endpoints that mutate or read
restricted data require a valid session (see §11). The column "Roles"
lists who can call the endpoint (`*` = any authenticated user, with
ownership/lease checks applied where applicable).

```
# --- Authentication ---
POST   /api/v1/auth/login              { login, pin }            *           # exchange login+PIN for session token
POST   /api/v1/auth/logout                                       *
GET    /api/v1/auth/me                                           *           # current user, effective roles, DCC pool

# --- API keys (per-user temporary keys, max lifetime 365d) ---
GET    /api/v1/apikeys                                           *           # own keys only (prefix + metadata, never plaintext)
POST   /api/v1/apikeys                 { name, expiresAt, scopes:[...] } *   # mint; returns plaintext ONCE in response
DELETE /api/v1/apikeys/{id}                                      *           # revoke (own key, or admin for any)

# --- User management ---
GET    /api/v1/users                                             admin
POST   /api/v1/users                   { login, pin, role }       admin
PUT    /api/v1/users/{id}/role         { role }                   admin       # change permanent role
POST   /api/v1/users/{id}/temp-role    { role, expiresAt }        admin       # grant temporary role
DELETE /api/v1/users/{id}/temp-role/{tempRoleId}                  admin       # revoke early
PUT    /api/v1/users/{id}/dcc-pool     { ranges:[{from,to},...] } admin       # assign DCC pool

# --- Vehicles ---
GET    /api/v1/vehicles                                           *           # all visible (own + leased + signalman-overridden)
POST   /api/v1/vehicles                { dccAddress, name, ... }  *           # register inside own DCC pool
PUT    /api/v1/vehicles/{addr}         { name, ... }              owner       # edit
DELETE /api/v1/vehicles/{addr}                                    owner

GET    /api/v1/vehicles/{addr}/cv/{n}                             owner       # CV read (lessee cannot)
POST   /api/v1/vehicles/{addr}/cv      { entries:[{n,v},...] }    owner       # CV write

# --- Vehicle functions (F0-F32; owner-only editing, lessee can only invoke) ---
GET    /api/v1/vehicles/{addr}/functions                          *           # resolved list (template OR vehicle rows; carries `source`)
PUT    /api/v1/vehicles/{addr}/functions/{num}  { name, icon, kind, position } owner   # upsert one slot; auto-detaches if linked
DELETE /api/v1/vehicles/{addr}/functions/{num}                    owner       # remove one slot; auto-detaches if linked
POST   /api/v1/vehicles/{addr}/functions/reorder { positions:[{num,position},…] } owner # auto-detaches if linked
POST   /api/v1/vehicles/{addr}/functions/detach                   owner       # explicit copy-on-write; idempotent
POST   /api/v1/vehicles/{addr}/functions/attach { templateId }    owner       # drop local rows, re-link to template
GET    /api/v1/function-icons                                     *           # closed catalogue of FunctionIcon values

# --- Vehicle templates (anyone creates; only owner or admin edits) ---
GET    /api/v1/vehicle-templates                                  *
GET    /api/v1/vehicle-templates/{id}                             *
POST   /api/v1/vehicle-templates       { name, description }      *
PUT    /api/v1/vehicle-templates/{id}  { name?, description? }    owner OR admin
DELETE /api/v1/vehicle-templates/{id}                             owner OR admin  # 409 unless ?cascade=true (§3a.6.4)

GET    /api/v1/vehicle-templates/{id}/functions                   *
PUT    /api/v1/vehicle-templates/{id}/functions/{num}             owner OR admin
DELETE /api/v1/vehicle-templates/{id}/functions/{num}             owner OR admin
POST   /api/v1/vehicle-templates/{id}/functions/reorder           owner OR admin

# --- Scripts (browser-side Python automation) ---
GET    /api/v1/scripts                                            *           # lists scripts the caller can SEE: owned + those attached to a vehicle/train the caller can drive (lessee). For lessee-visible rows the `source` field is omitted.
GET    /api/v1/scripts/{id}                                       owner       # full source; lessee gets 403 even with an active lease
POST   /api/v1/scripts                 { name, source, runtime, icon, description? } *           # creates a script owned by the caller; source ≤ 64 KiB, runtime ∈ {micropython, pyodide}
PUT    /api/v1/scripts/{id}            { name?, source?, runtime?, icon?, description? } owner   # bumps `version`; fan-out `script.changed` to live throttles
DELETE /api/v1/scripts/{id}                                       owner       # also drops every ScriptAttachment row

# Script attachments – a script may be bound to a vehicle XOR a train.
GET    /api/v1/scripts/{id}/attachments                           *           # owner sees all; lessee sees only attachments to vehicles/trains they can drive
POST   /api/v1/scripts/{id}/attachments { vehicleAddr? , trainId? , position? } owner            # exactly one of vehicleAddr / trainId required; 422 otherwise
DELETE /api/v1/scripts/{id}/attachments/{attachmentId}            owner

# Reverse listing: scripts visible on a given throttle (used by the UI to populate the script-button row alongside F0..F32)
GET    /api/v1/vehicles/{addr}/scripts                            * (driving authority)
GET    /api/v1/trains/{id}/scripts                                * (driving authority)

# --- Leasing ---
POST   /api/v1/vehicles/{addr}/lease   { toUserId, expiresAt }    owner
DELETE /api/v1/vehicles/{addr}/lease                              owner       # revoke active lease
POST   /api/v1/trains                  { name, members:[...] }    *           # only own vehicles
POST   /api/v1/trains/{id}/lease       { toUserId, expiresAt }    owner
DELETE /api/v1/trains/{id}/lease                                  owner

# --- Interlockings ---
GET    /api/v1/interlockings                                      *           # FILTERED to the caller's active party (only whitelisted IDs)
POST   /api/v1/interlockings/{id}/join                            signalman   # join (becomes active session); requires interlocking ∈ active party
POST   /api/v1/interlockings/{id}/leave                           signalman

# --- Layouts (catalogue of `makiety`) ---
GET    /api/v1/layouts                                            *           # list (name + connection type only; admin sees full Connection)
GET    /api/v1/layouts/{id}                                       admin       # full details incl. Connection
POST   /api/v1/layouts                 { name, connection }       admin
PUT    /api/v1/layouts/{id}            { name, connection }       admin
DELETE /api/v1/layouts/{id}                                       admin       # 409 if any Party still references it

# --- Parties (modeling events) ---
GET    /api/v1/parties                                            *           # list shown right after login; rows carry `canEdit:bool` for admin badge and `layoutPickedPerSession:bool` (true only for `default`)
GET    /api/v1/parties/{id}                                       *
POST   /api/v1/parties                 { name, layoutId }         admin       # layoutId REQUIRED – only the bootstrap `default` row is allowed to have a NULL layout (you cannot create another such row)
PUT    /api/v1/parties/{id}            { name?, layoutId? }       admin       # 422 on attempt to set layoutId=null on a non-default party
DELETE /api/v1/parties/{id}                                       admin       # cannot delete `default`

POST   /api/v1/parties/{id}/join                                  *           # enter party.
                                                                              #   - non-default: fails if its layout is unreachable
                                                                              #   - default:     ALWAYS succeeds (session.LayoutID starts nil; driver picks later)
POST   /api/v1/parties/{id}/leave                                 *           # equivalent to closing all drive sessions for that party

# Party-scoped signalmen
GET    /api/v1/parties/{id}/signalmen                             *
POST   /api/v1/parties/{id}/signalmen  { userId, expiresAt? }     admin       # grant signalman role inside this party
DELETE /api/v1/parties/{id}/signalmen/{userId}                    admin

# Party-scoped interlocking whitelist
GET    /api/v1/parties/{id}/interlockings                         *
POST   /api/v1/parties/{id}/interlockings { interlockingId }      admin OR signalman-of-this-party
DELETE /api/v1/parties/{id}/interlockings/{interlockingId}        admin

# --- Audit log (admin only, append-only) ---
GET    /api/v1/audit-log                                          admin       # filterable: ?action=&actor=&objectType=&objectId=&partyId=&since=&until=&limit=&offset=
GET    /api/v1/audit-log/{id}                                     admin

# --- System ---
GET    /api/v1/system/status                                      *           # command station info FOR THE CALLER'S ACTIVE PARTY
```

Takeover, throttle and radio are **all WebSocket-only** because they are
short, frequent, and event-driven.
