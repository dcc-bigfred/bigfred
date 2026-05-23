## 4. Communication Protocol

Two transports carry the application's traffic: REST for CRUD-like,
idempotent reads/writes, and a single WebSocket for everything that is
short, frequent and event-driven (throttle, takeover, radio,
dead-man's switch, scripts).

## Subsections

1. [REST](./01-rest.md) — full endpoint catalogue under `/api/v1`
2. [WebSocket](./02-websocket.md) — actions, events, envelope format
3. [Takeover state machine](./03-takeover-state-machine.md)
4. [Radio – delivery rules](./04-radio-delivery.md)
5. [Drive Session & Dead-Man's Switch](./05-drive-session-dms.md)
