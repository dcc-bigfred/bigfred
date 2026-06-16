# `pkgs/bigfred/contract`

Shared **Redis contract** between `loco-server` (`pkgs/bigfred/server`) and
the `dcc-bus` daemon (`pkgs/bigfred/dcc-bus`).

The two processes coordinate over Redis only — no shared SQLite, no direct RPC.
This package is the single source of truth for **what** they exchange: key and
channel names, JSON payload shapes, and small helpers that build concrete keys
and wire-format bytes from primitive Go values.

## What lives here

| File | Responsibility |
|------|----------------|
| [`allowedvehicles.go`](allowedvehicles.go) | Layout roster keys, snapshot types (`AllowedVehicles`, `DefinedTrains`, …), `Marshal` / `Unmarshal*`, and payload builders. |
| [`trains.go`](trains.go) | Train driving helpers (`LeadingMember`, `CanDrive`, `EffectiveMemberSpeed`, `MaxSpeedForSpeedSteps`, `TrainSetSpeedWire`). |
| [`locostate.go`](locostate.go) | Per-loco state key (`loco:state`), `LocoStateWire`, and `BuildLocoStatePayload`. |
| [`dccbus.go`](dccbus.go) | Command/event channels, port-pool hash, `EnvelopeWire`, command intents (`LocoSetSpeedWire`, `LocoSetFunctionWire`), and envelope/port builders. |

## Key templates and builders

Constants ending in `Tmpl` are `fmt` format strings — the canonical spelling of
a Redis key or channel. Call the paired `*Key` / `*Channel` / `*Field` function
to obtain the concrete string for a given `(layoutID, commandStationID, addr, …)`
tuple.

Example:

```go
key := contract.AllowedVehiclesKey(layoutID) // bigfred:layout:7:allowed_vehicles
ch  := contract.DccBusEventChannel(layoutID, csID) // dcc-bus:evt:7:2
```

Never duplicate these literals in server or dcc-bus code — add a new `*Tmpl`
constant and builder here first, then use it from both sides.

## Payloads from primitive types

Snapshot structs are built from plain Go values (`uint`, `uint16`, `string`,
slices of structs, …) that loco-server already has after loading SQLite rows.
`dcc-bus` never opens the database; it unmarshals the same JSON the server
published.

Typical publisher flow (server):

```go
snap := contract.AllowedVehicles{
    LayoutID:  layoutID,
    UpdatedAt: contract.NowMS(),
    Vehicles:  []contract.AllowedVehicle{ /* from domain rows */ },
}
raw, err := contract.Marshal(snap)
// SET + PUBLISH on contract.AllowedVehiclesKey(layoutID)
```

Typical consumer flow (dcc-bus):

```go
snap, err := contract.UnmarshalAllowedVehicles(msg.Payload)
```

Command intents carried on `dcc-bus:cmd:*` (server → daemon) — `LocoSetSpeedWire`
and `LocoSetFunctionWire` — live here because they are part of the cross-process
contract; the dcc-bus WebSocket reuses the same structs for client → daemon
`loco.setSpeed` / `loco.toggleFn`, and additionally hosts `train.setSpeed` via
`TrainSetSpeedWire` (daemon-side fan-out only — not on the command channel).
State snapshots (`LocoStateWire`) and roster snapshots are the other payload
families defined in this package. Frames that never leave the dcc-bus
WebSocket (subscribe, ack, errors, the welcome frame) stay in
`pkgs/bigfred/dcc-bus/protocol`.

## What does **not** belong here

- Redis client wiring, pub/sub loops, or TTL policy — `pkgs/bigfred/dcc-bus/state`,
  `pkgs/bigfred/server/service`.
- Authorization rules or domain entities — `pkgs/bigfred/server/security`,
  `pkgs/bigfred/server/domain`.
- HTTP / WebSocket transport — `pkgs/bigfred/server/http`, `pkgs/bigfred/dcc-bus/ws`.

## Consumers

Both `loco-server` and `dcc-bus` import this package. It must stay free of
imports from either side so the dependency graph stays acyclic.

## Further reading

- [§7e.3 State & Redis cache](../../../docs/bigfred/architecture/16-dcc-bus/03-state-and-redis.md) — behavioural spec for each key and channel.
- [§3.2 Contract package](../../../docs/bigfred/architecture/04-repository-layout.md#32-contract-package-pkgsbigfredcontract) — where `contract` sits in the repository layout.
