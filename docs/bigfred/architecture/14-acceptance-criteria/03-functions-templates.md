### 10.2a Vehicle functions and templates (M3)

- The owner of a vehicle can register **between 0 and 32 functions**
  on it, each with a function number in `F0`–`F31`, a title (*tytuł*,
  field `name`), an icon from the catalogue (§3a.8) and a
  `latched` (*Stała*) / `momentary` (*Chwilowa*) kind. A request with
  `num=32` or `num=-1` is rejected with `422`.
- The same `num` cannot be registered twice on one vehicle (`409`
  conflict on the unique index).
- The closed list of icons (67 entries, §3a.8) is served by
  `GET /api/v1/function-icons` and matches an asset bundle on the
  frontend; no other icon string is accepted by `PUT .../functions/{num}`.
- On the vehicle catalogue (`/vehicles`, §6.3e) each owned row shows
  **Edytuj funkcje** next to **Edytuj**; clicking it opens
  `/vehicles/{addr}/functions` where the owner can add, edit, remove and
  drag-sort functions. Reordering persists via `POST …/functions/reorder`
  and the throttle button row reflects the new `position` order without
  a separate configuration step.
- Every function registered for a vehicle appears as a button in throttle
  mode for that vehicle (`<FunctionButtons>`), in `position` order.
- A user can create a vehicle template and pick it when registering a
  new vehicle. The new vehicle's `GET .../functions` returns the
  template's list with `source: "template"`, and there are no
  `dcc_functions` rows with `vehicle_id` set for that vehicle.
- Editing **any** function on a template — renaming a slot, changing
  its icon, adding or removing a slot, reordering — is **immediately
  visible** to every linked vehicle's throttle (the server emits
  `vehicle.functionsChanged` to every subscriber).
- The first edit a user performs on **their vehicle's** functions
  detaches the vehicle: `dcc_functions` rows with `vehicle_id` are
  inserted as copies of the template rows in the same transaction,
  `FunctionsDetachedAt` is set,
  and the requested edit is applied. The state change happens
  **exactly once per vehicle**.
- After detachment, further template edits do **not** affect the
  vehicle. The vehicle's UI keeps showing exactly the configuration
  the user left it in.
- An owner can request `POST .../functions/attach { templateId }` to
  drop their local edits and re-sync to the named template's current
  state. The local rows are deleted and `FunctionsDetachedAt` is reset
  to `nil`.
- A lessee currently holding an active lease on a vehicle can
  **invoke** the registered functions (the throttle's icon buttons
  fire DCC `Fn` packets) but cannot edit the definitions: any
  `PUT/POST/DELETE` to `.../functions` returns `403` with
  `only_owner_can_edit`.
- Deleting a template still referenced by a vehicle returns `409
  Conflict`. The same request with `?cascade=true` detaches every
  linked vehicle (materialising the function list at the time of
  cascade) and clears `template_id` on every detached-with-this-lineage
  row before deleting the template.
