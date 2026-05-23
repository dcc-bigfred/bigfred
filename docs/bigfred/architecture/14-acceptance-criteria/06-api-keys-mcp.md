### 10.4 API keys + MCP (M6)

- A user can mint an API key via the UI; the plaintext is shown exactly
  once and never returned by any subsequent API call. The list view
  shows the key prefix and metadata only.
- An attempt to mint a key with `expiresAt` further than **365 days**
  in the future is rejected with a clear validation error; a key
  created with a shorter lifetime stops being accepted exactly when
  `expiresAt` passes, without any manual revocation.
- Revoking a key invalidates it within seconds: an in-flight MCP
  session is closed and the next REST call returns `401`.
- Using `Authorization: Bearer rb_…` on a REST endpoint behaves
  identically to a logged-in session of the key's owner, except that
  the call is additionally limited by the key's `scopes`.
- A local MCP client (e.g. Claude Desktop or Cursor) configured with
  `loco server --mcp-stdio` and the env variable `BIGFRED_API_KEY` can
  list locomotives, set a locomotive's speed and send a radio phrase –
  all through the same `LocoService` / `RadioService` that the human
  UI uses.
