## Goals

### Platform goals

1. Be reachable from any modern browser on mobile and desktop.
2. Provide a real-time control surface for locomotives: speed, direction,
   functions, CV read/write and live status feedback from the command
   station.

### Functional goals (multi-user operations)

The application is a multi-user "operating session" tool, not just a
throttle. The following requirements are first-class concerns of the
domain model and the API:

1. **Authentication** – every user has a `login` and a numeric `PIN`
   (short PIN is chosen on purpose: easy to type on a phone in a club
   room; protected by hashing + rate-limiting, see §11).
2. **Roles** – three roles exist in the system:
   - `driver` – operates vehicles and trains,
   - `signalman` – occupies a signal box (interlocking) and directs
     traffic,
   - `admin` – manages users, roles and DCC address allocations.
3. **Role management** – `admin` can:
   - assign or revoke permanent roles,
   - grant **temporary roles** with an explicit expiry timestamp; the
     grant is automatically removed when it expires (no manual cleanup),
   - assign each user a **pool of DCC addresses** they are allowed to
     register vehicles under.
4. **Vehicle registration** – any user can register their own vehicles
   in the application **only within their assigned DCC pool**.
5. **Vehicle control** – a user can drive:
   - vehicles they own,
   - vehicles currently leased to them by another user (see below).
6. **Vehicle leasing** – a user can lend a vehicle to another user
   **for driving only** (no edit / no CV writes). Properties of a lease:
   - explicit `expires_at` timestamp; lease auto-expires after that
     moment with no manual action,
   - can be revoked early by the owner at any time,
   - lessee never inherits ownership or edit rights.
7. **Trains** – a user may create a **train** (`skład`) made up of at
   least one vehicle. The train is owned by the user that created it
   and can be leased exactly like a single vehicle (same `expires_at`
   / revoke semantics). The **train control view shows the same speed
   slider as the single-vehicle view**: moving the slider sets the
   speed of **every member vehicle in lock-step**, with members whose
   `Reversed` flag is `true` driven in the opposite direction so the
   whole consist moves as a rigid unit. Function buttons and scripts
   remain per-vehicle (each member keeps its own F0–F32 row); only
   the throttle is consolidated.
8. **Interlockings / Signal boxes** – the system models physical
   `interlockings` (`nastawnie`). At any moment **at most one signalman
   can occupy a given interlocking** in order to direct traffic from
   there.
9. **Takeover by signalman** – an occupying signalman may request to
   take control of a driver's vehicle or train. The flow is:
   - signalman emits `takeover.request`,
   - driver receives `takeover.requested` and has **15 seconds** to
     reject it,
   - on `takeover.reject` from the driver, or if the driver leaves the
     session, the request is cancelled,
   - if the 15-second window elapses with no rejection, the signalman
     becomes the active controller; ownership is unchanged, but driving
     authority is moved.
10. **Radio ("walkie-talkie") between signalmen and drivers** – the app
    provides a built-in messaging channel between drivers and signalmen
    based on a closed set of **standard radio phrases** (for example
    `STOPPED_AT_SIGNAL_READY_TO_ENTER`, `ENTRY_PERMITTED`,
    `CANCEL_ROUTE`, `ACK`). Messages are short, structured, addressable
    and delivered over the same WebSocket connection.
11. **Programmable access (API keys + built-in MCP server)** – any user
    can mint **temporary API keys** scoped to their own permissions:
    - configurable lifetime up to a **hard maximum of 365 days**;
    - the plaintext key is shown to the user **exactly once** at
      creation time, only a hash is persisted;
    - keys can be revoked at any moment and auto-expire on
      `expires_at`;
    - keys are accepted by both the REST API (header
      `Authorization: Bearer rb_…`) and the **built-in MCP server**
      that the same Go binary exposes;
    - MCP exposes a curated tool surface (list locomotives, set speed,
      toggle function, send radio phrase, …) so AI assistants, IDE
      agents and automation scripts can drive the same domain via
      Anthropic's [Model Context Protocol](https://modelcontextprotocol.io/).
12. **Parties (modeling events) and the default workspace** – the
    application is multi-tenant in the soft sense that all users live
    in the same database, but every driving session happens inside a
    **party** (`impreza`):
    - immediately after login the UI presents a **list of parties**;
      the user either joins one or enters the system-provided
      **`default`** workspace, which is always present;
    - **`admin` creates parties**, **any user may join any party**;
    - **a party has no end date** – it stays in the catalogue until an
      admin deletes it;
    - each party owns its own **party-scoped signalmen list**: an
      admin may grant the `signalman` role to a user **only inside one
      specific party**, and the user gains signalman powers exclusively
      while active in that party (see §7a.2 for how this changes the
      effective-roles computation);
    - each party owns its own **interlocking whitelist**: both `admin`
      and any signalman of the party may add interlockings to it;
      **only the whitelisted interlockings are visible to drivers
      currently in that party**;
    - the party list rendered right after login shows a **settings
      icon next to every party**, visible only to users with the
      `admin` role.
13. **Layouts catalogue (`makiety`)** – the physical model layout each
    party runs on is a first-class entity:
    - the system maintains a **catalogue of layouts**; each layout has
      a name and a **connection definition** describing how the
      backend reaches the command station:
      a) a **physical LocoNet socket** (serial / TTY device),
      b) **Z21 over network** (host + port),
      c) **LocoNet over Network** (host + port);
    - **only `admin` may create, edit or delete layouts**;
    - **every non-`default` party must be assigned exactly one
      layout** at creation time. A non-default party with no layout
      is unreachable and joining it fails with a clear error;
    - the **`default` party is the one explicit exception**: it has
      **no fixed layout**. Instead, in `default` the **driver picks a
      layout from a dropdown** inside the vehicle control view, and
      that pick becomes the active layout for the current drive
      session until the driver picks a different one. This makes
      `default` the workspace where any user can experiment with any
      available layout without an admin first having to create a
      dedicated party for it.
14. **Audit log** – every significant state change is recorded in an
    **append-only audit log**. The scope is deliberately narrow and
    covers the operationally interesting events:
    - **vehicle leasing** – grant, revoke, auto-expire;
    - **train leasing** – grant, revoke, auto-expire;
    - **vehicle create / edit / delete**;
    - **train create / edit / delete**;
    - **"driver fell asleep"** (`maszynista zasnął`) – the
      dead-man's switch firing (§4.5), with the list of affected
      vehicles attached;
    - **layout create / edit / delete** (`makieta`);
    - **party create / edit / delete** (`impreza`).

    Every entry MUST carry the following fields:

    | Field          | Type         | Notes                                                            |
    |----------------|--------------|------------------------------------------------------------------|
    | action type    | `string`     | e.g. `vehicle.leased`, `session.emergency_executed`              |
    | user name      | `string`     | `user.login` **at the moment of the event** (denormalized)        |
    | user ID        | `uint`       | `user.id`                                                         |
    | date           | `time.Time`  | UTC; persisted with millisecond precision                         |
    | object ID      | `uint`       | id of the affected vehicle/train/layout/party/session             |
    | object name    | `string`     | e.g. vehicle name, train name, layout name (denormalized)         |

    Denormalization of `user name` and `object name` is intentional:
    deleting or renaming a user/vehicle later **must not rewrite
    history**. The audit log is read-only for everyone (no DELETE/UPDATE
    endpoints) and visible only to `admin`. See §3a.5 for the entity,
    §4.1 for the REST surface and §10.6 for the acceptance criteria.
15. **Vehicle functions (`F0`–`F32`)** – every vehicle exposes a
    user-curated list of DCC functions that drivers can toggle from
    the throttle UI:
    - the underlying DCC function range is **`F0`–`F32`** (33
      possible slots); a given vehicle may register **any number** of
      slots from that range (zero, several, or all of them) – the
      owner registers only those that physically exist on the
      decoder;
    - each registered function carries: the function number, a
      user-given **name**, an **icon** picked from a closed catalogue
      shipped with the frontend (e.g. *high horn*, *low horn*,
      *headlight*, *taillight*, *shunting mode*, *engine start*,
      *bell*, *cab light*, *coupler*, *smoke*, …), and a **kind**
      (`latched` = toggle / `momentary` = press-and-hold);
    - **only the vehicle's owner** may edit the function definitions;
      lessees and signalmen who took the vehicle over may **invoke**
      functions while they have driving authority, but never edit the
      list.
16. **Vehicle templates with copy-on-write inheritance** – the system
    has a catalogue of **vehicle templates** (`szablony pojazdów`)
    that pre-define a function list for a class of vehicles
    (e.g. "PKP ET22", "DB BR 218", "Bachmann 0-6-0 with sound"):
    - any user can create templates; the **owner** of a template (or
      admin) may edit it;
    - when registering a new vehicle, the user may optionally pick a
      template; the new vehicle is then **linked** to that template
      and its function list is **virtual** – served live from the
      template at read time;
    - **as long as the user does not edit a function on the
      vehicle**, the vehicle's function list **stays in sync** with
      the template: adding, renaming, removing or re-icon-ing
      functions in the template is immediately visible on every linked
      vehicle;
    - **the first edit the user makes to a function on their vehicle
      detaches the vehicle** from the template with **copy-on-write**:
      the entire template function list is snapshotted into the
      vehicle's own rows in a single transaction, and the requested
      edit is applied on the copy. Future template changes no longer
      affect this vehicle;
    - the user can also explicitly **detach** (manual copy) or
      **re-attach** (drop local edits, re-sync to template's current
      state) via dedicated endpoints.
17. **Persistent drive session with dead-man's switch** – the
    WebSocket connection is treated as a **drive session**, not just a
    transport:
    - the server tracks per-user sessions with a heartbeat
      (WS `ping`/`pong` every 10 s, plus an explicit application-level
      `ping` from the client every few seconds);
    - if the connection is closed (app shut down, tab closed, browser
      crash) or if heartbeats stop arriving for longer than a
      configurable **grace period** (default 5 s), the session is
      declared **lost**;
    - when the **last remaining session of a user** is lost, the server
      executes the user's configured **emergency action**, which by
      default is **stop all vehicles currently under that user's
      active control** (`SetSpeed(0)` on every owned + leased-in +
      taken-over vehicle being driven from any of their sessions);
    - other emergency actions are available per user/session preference
      (`release_my_leases`, `none` for testing, `estop_all` reserved
      for admins);
    - a successful reconnect within the grace window **cancels** the
      pending emergency.

18. **Scripts – server-side JavaScript automation attached to vehicles
    and trains** – the app exposes a **Scripts** tab where any user
    can author short **JavaScript (ECMAScript 5.1+)** programs that
    automate driving. Architecturally scripts live entirely on the
    backend:
    - scripts are stored as plain text and **executed server-side**
      in a sandboxed [Goja](https://github.com/dop251/goja) VM
      (pure-Go ES5.1 engine, no cgo). Each running script gets its
      **own `*goja.Runtime` owned by exactly one goroutine** (Goja
      VMs are explicitly **not goroutine-safe**, see Goja's FAQ);
    - to protect the main server, the Goja VMs do **not** run inside
      the `server` process. They run inside a separate
      **`scripts-executor` process** spawned by the server. The
      executor reuses the **same Go codebase** (same `pkgs/server`
      domain, services, security layer) – the only difference is the
      `main()` entry point: instead of opening REST/WS sockets it
      opens an internal RPC channel and waits for run requests. A
      runaway script (infinite loop, OOM, panic in a Go binding,
      Goja itself misbehaving) takes down the executor, **never the
      throttle server**;
    - the frontend never sees JavaScript. It hands the user's source
      to the server over REST and presses a play button over WS. The
      server forwards the run request to the executor over the
      internal RPC channel and proxies events (`log`, `runStarted`,
      `runStopped`) back to the frontend;
    - the script is **attached to a single vehicle or a single
      train**, the same way functions are attached to vehicles, and
      gets its own **icon picked from the function-icon catalogue**
      so it can be invoked from the throttle UI just like an `F0`–
      `F32` button;
    - the executor exposes a small, **deliberately limited DSL** that
      operates **only within the attached scope** – the canonical
      example:

        ```javascript
        const loco = findFirstLoco();   // first member with Kind=loco in the train,
                                        // or the attached vehicle iff it is a loco
        loco.setSpeed(10);              // 0..126, same semantics as loco.setSpeed WS

        const wagon = findByDCCAddr(815); // lookup is RESTRICTED to attachment scope
        wagon.funcOn(5);                  // F5 ON  (same as vehicle.setFunction)
        sleep(5);                         // blocks ONLY this script's goroutine
                                          // (not the server, not the executor's other VMs)
        wagon.funcOff(5);                 // F5 OFF
        ```

    - every DSL call is a Go binding that goes through the **same
      `LocoService` / `TrainService` and the same security policy
      layer (§7a.3) as a manual throttle press**, so authorization,
      lease checks, takeover handoff, audit and the dead-man's switch
      contract apply unchanged: a script can **never do anything its
      user could not do manually**;
    - scripts are **edited only by their owner**; a lessee of a
      vehicle/train sees the script icon on the throttle and can
      **run** the script (their driving authority is the limit), but
      cannot view or change its source;
    - **start, stop and progress** of a running script are mirrored
      across all of the owner's open sessions (phone + desktop), so
      tapping "stop" on the phone halts the script regardless of
      where it was started. **Stop** is implemented server-side via
      `vm.Interrupt(...)` from a sibling goroutine in the executor.

These functional goals drive the domain model (§3a), the REST surface
(§4.1), the WebSocket protocol (§4.2), the **drive-session contract
and dead-man's switch (§4.5)**, the **party / layout addressing rules
(§3a.4)**, the **audit log (§3a.5)**, the **vehicle functions and
template inheritance (§3a.6)**, the **server-side scripting model
in the sibling `scripts-executor` (§3a.7)**, the authorization
rules (§7a) and the MCP integration (§7b).
