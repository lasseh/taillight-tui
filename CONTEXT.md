# taillight-tui domain glossary

**Feed** — one live SSE stream of one event type from the taillight API
(`srvlog`, `netlog`, `applog`), plus the rules for its lifetime: open lazily,
reopen only when the server-side filter changes, close a replaced stream,
drain decoded events per tick. Implemented by `internal/tui/feed`; log tabs
and the dashboard's recent-events panels each hold feeds, and nothing opens
an SSE stream except through a feed.

**Log tab** — a feed paired with the log view it fills (table, search input,
filter popup, detail sidebar) and its toast rule. The App holds one per log
tab and treats them uniformly.

**Filter** — the server-side narrowing a feed's stream runs with, encoded as
SSE query params. The log view owns the search input; each feed's filter owns
its extra fields, keyed by query param name (`hostname`, `programname`,
`severity_max`, `facility` for srvlog/netlog; `service`, `component`, `host`,
`level` for applog).

**Running filter** — the params the open stream was started with. Status bar
pills show the running filter, never half-typed search input.

**Backfill** — the recent events the API replays when a stream opens. A
(re)opened stream starts on an empty view and backfill repopulates it.
