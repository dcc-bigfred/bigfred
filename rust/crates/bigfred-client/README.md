# bigfred-client

<p align="center">
  <img src="logo.png" alt="BigFred" width="200">
</p>

HTTP, OAuth drop-in, reverse-proxy helpers, and dcc-bus WebSocket client for [BigFred](https://github.com/dcc-bigfred/bigfred). **No axum:** the host owns HTTP handlers and maps [`Error`](https://docs.rs/bigfred-client) onto its envelope. Config is a snapshot ([`BigFredConfig`](https://docs.rs/bigfred-client/latest/bigfred_client/struct.BigFredConfig.html)); the host refreshes it on reload.

The crate’s `reqwest` build has **no TLS** features. Talk to BigFred over loopback HTTP on the hub.

Version matches the BigFred release tag (`vX.Y.Z` → crate `X.Y.Z`).

## Install

```toml
[dependencies]
bigfred-client = "0.0"
```

From git (`master`):

```toml
[dependencies]
bigfred-client = { git = "https://github.com/dcc-bigfred/bigfred.git", branch = "master" }
```

Cargo resolves the package by name under `rust/crates/bigfred-client`.

## Config

This crate does **not** read wizard JSON or `$DATA_DIR`. The host builds a [`BigFredConfig`](https://docs.rs/bigfred-client/latest/bigfred_client/struct.BigFredConfig.html):

```rust
use std::path::PathBuf;
use bigfred_client::BigFredConfig;

let data_dir = std::env::var("BIGFRED_DATA_DIR")
    .or_else(|_| std::env::var("DATA_DIR"))
    .unwrap_or_else(|_| "/data".into());

let cfg = BigFredConfig {
    api_base: "http://127.0.0.1:8080".into(),
    ws_base: "ws://127.0.0.1:8080".into(),
    sso_client_id: "my-app".into(),
    redirect_uris: vec![
        "http://my-app.local:8091/auth/callback".into(),
        "http://localhost:8091/auth/callback".into(),
    ],
    oauth_dropin_dir: PathBuf::from(data_dir).join("etc/bigfred/oauth-clients"),
    oauth_display_name: "My App".into(),
    fixed_dcc_bus: None, // or Some((station_id, layout_id))
};
```

Use a shared `reqwest::Client` (JSON, no TLS):

```rust
let http = reqwest::Client::builder()
    .timeout(std::time::Duration::from_secs(30))
    .build()?;
```

## OAuth (confidential client)

BigFred watches `$DATA_DIR/etc/bigfred/oauth-clients/` and loads `{sso_client_id}.json`. The **browser never sees the client secret**. The host seeds the drop-in, the SPA starts authorize, then the host exchanges the code.

### 1. Seed the drop-in at boot

```rust
use bigfred_client::oauth;

let path = oauth::ensure_dropin(&cfg)?;
```

- Missing file: writes a new client (`enabled: true`, random 64-hex `clientSecret`, `shareSession: false`).
- Existing file: keeps the secret; **merges** `cfg.redirect_uris`.
- Unix: directory `0750`, file `0640`, group `bigfred` so the daemon can read it. Touch the file when URIs change so BigFred’s inotify reloads.

### 2. Start authorize in the browser

Redirect the user to BigFred (public URL, not loopback):

```text
GET {publicBigFred}/api/v1/auth/oauth/authorize
  ?client_id={sso_client_id}
  &redirect_uri={one of cfg.redirect_uris}
  &state={csrf}
  &response_type=code
  &layout_id={layoutId}
```

Keep `state` in the browser (e.g. `sessionStorage`) and check it on the callback. Do not put the client secret in any public config JSON.

### 3. Exchange the code on the host

The SPA posts `{ code, redirectUri }` to **your** backend. The backend allowlists `redirect_uri`, then:

```rust
use bigfred_client::oauth;

let token = oauth::exchange_token(&http, &cfg, code, redirect_uri).await?;
// token.access_token, token.token_type, token.expires_at
```

That calls `POST {api_base}/api/v1/auth/oauth/token` with `grantType: authorization_code` plus `clientId` / `clientSecret` from the drop-in. Return `TokenResponse` to the SPA (access token only).

If the drop-in is missing at exchange time, `exchange_token` seeds it and retries.

## Same-origin HTTP proxy

Forward `/api/v1/*` (except your own routes) to BigFred. Copy only the allowlisted request headers:

```rust
use bigfred_client::proxy;
use bigfred_client::IMPERSONATE_HEADER;

let names: Vec<_> = proxy::forwarded_request_header_names().collect();
// "authorization", "content-type", "accept", IMPERSONATE_HEADER

let resp = proxy::forward(
    &http,
    &cfg.api_base,
    "GET",
    "/api/v1/version",
    None,
    &[],
    &[],
).await?;
```

`IMPERSONATE_HEADER` is `x-bigfred-impersonate-as` (participant login).

## PIN check (no driver JWT)

```rust
use bigfred_client::apis;

apis::verify_pin(&http, &cfg, "alice", "1234", layout_id).await?;
```

On success the minted session is discarded so a kiosk never keeps a driver JWT.

## dcc-bus WebSocket

Two sockets: **programming** (organizer JWT) and **drive** (organizer JWT + impersonation).

```rust
use std::sync::Arc;
use tokio::sync::RwLock;
use bigfred_client::DccBusClient;

let cfg = Arc::new(RwLock::new(cfg));
let dcc = DccBusClient::new(cfg, http);

dcc.ensure_connected(organizer_token).await?;
dcc.ensure_connected_to(organizer_token, 3).await?;
dcc.request_to(
    organizer_token,
    3,
    "loco.cvRead",
    serde_json::json!({ "address": 0, "cvs": [1], "mode": "prog" }),
)
.await?;
dcc.ensure_drive(organizer_token, "alice").await?;
dcc.pulse_function(organizer_token, "alice", 3, 2, 500).await?;
```

The dcc-bus daemon must be up (open the layout in BigFred once). Otherwise `Error::DccBusUnreachable`.

## License

MIT
