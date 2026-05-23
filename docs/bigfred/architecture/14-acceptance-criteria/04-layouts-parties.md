### 10.2b Layouts and parties (M4)

- A fresh installation seeds exactly one party named `default` with
  **no layout assigned** (`layout_id IS NULL`). Admins can rename it
  but **cannot delete** the `default` party and **cannot pin a layout
  to it** through `PUT /api/v1/parties/{default}` – attempting to set
  `layoutId` on `default` returns `422` with `default_party_layout_immutable`.
- Only admins can create, edit or delete layouts. A non-admin user
  calling `POST /api/v1/layouts` gets `403`.
- Creating a **non-default** party requires `layoutId`; a request
  omitting it or pointing to a deleted layout is rejected with `422`
  and a clear error message. Creating a second party with
  `layoutId = null` is not possible — only the bootstrap `default`
  row may exist with a `NULL` layout (DB CHECK + service).
- Attempting to delete a layout that is still referenced by at least
  one non-default party returns `409 Conflict`. The `default` party
  never references a layout and therefore never blocks deletion.
- Right after login the user sees the **party list screen**; every row
  shows a settings icon **only for users with the `admin` role**.
  Non-admin users do not see the icon (and the endpoints behind it
  return `403` if probed). Each row also exposes a
  `layoutPickedPerSession: true|false` flag so the UI can render an
  obvious "Pick on entry" badge on the `default` row.
- Joining a **non-default** party whose layout is unreachable on its
  configured endpoint fails fast (`502 Bad Gateway` from `POST
  /api/v1/parties/{id}/join`) and no drive session is opened.
- Joining the **`default`** party **always** succeeds and opens a
  drive session with `LayoutID = nil`. Until the driver picks a
  layout, **every throttle command returns `layout_not_selected`**
  and the UI keeps the throttle disabled.
- Inside `default`, the **vehicle control view shows a layout
  dropdown** populated from `session.opened.availableLayouts`.
  Picking a layout fires `session.setLayout { layoutId }`; the server
  replies with `session.layoutChanged` and the throttle becomes
  enabled. Picking a different layout later is allowed and triggers a
  controlled context switch: the previous layout's vehicles receive
  `SetSpeed(0)` first (same code path as the dead-man's switch
  emergency plan), then the session re-points to the new layout.
- Calling `session.setLayout` from a non-default party returns
  `ack { ok:false, error:"layout_already_pinned" }`; the layout is
  immutable for the lifetime of a non-default drive session.
- The admin can grant the `signalman` role to a user **scoped to one
  specific party**; that user only has signalman powers while their
  active session is in that party. Switching to a different party
  removes the powers immediately.
- Both admins and signalmen of a party can add interlockings to that
  party's whitelist; `GET /api/v1/interlockings` for a driver in that
  party returns exactly the whitelisted set, and interlockings not on
  the whitelist are invisible in the UI. This applies to `default` as
  well – its whitelist starts empty.
- A driver in party A and a driver in party B (both on different
  layouts) can drive simultaneously; their commands reach independent
  `Station` instances (one per layout) without interference. Two
  drivers in `default` can pick the **same** layout — they share the
  DCC bus and the UI shows a "shared bus" chip on both throttles.
