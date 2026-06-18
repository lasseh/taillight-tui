# Follow-ups

Improvements deferred during the extraction from the main Taillight repo. None
block correctness today; they were left out of the migration to keep it
behaviour-preserving and reviewable. Roughly ordered by value.

## Code consolidation
- **Generic SSE drain helper.** `internal/tui/app_streams.go` has three
  near-identical `drain*SSE` functions differing only in the unmarshal type.
  Collapse to one `drainSSE[T any](stream *client.SSEStream, max int) ([]T, int)`.
- **Generic syslog feed view.** `view/srvlog` and `view/netlog` are ~330 lines
  of byte-for-byte duplicated columns/filter code (netlog aliases the srvlog
  event type). Extract a shared, parameterised syslog feed package and keep thin
  per-feed wrappers.
- **Generic feed client.** `client/srvlog.go` and `client/netlog.go` duplicate
  List/Get/Hosts/Programs/Summary, differing only in the path prefix.
- **Shared attr formatter.** `formatAttrsInline` is duplicated between
  `view/dashboard` and `view/applog`. Extract one helper; on marshal error fall
  back to `fmt.Sprintf("%v", val)` rather than emitting an empty value.

## Correctness / UX
- **Display-width truncation.** `truncate`/`padRight` in the column builders and
  the toast component slice strings by byte index, which can split multi-byte
  runes and miscompute width. Move to a rune- and display-width-aware helper
  (e.g. via lipgloss/ansi width) shared across all feeds.
- **Surface dashboard load failures.** `view/dashboard` now uses a timeout
  context for `loadStats`, but a failed refresh is still swallowed in `Update`.
  Show a "last refresh failed" indicator and/or keep the previous data.
- **SSE field parsing.** `client/sse.go` only matches the `field: value` form
  (with a space). Accept the spec-legal no-space form (`id:5`) and track
  `lastID` on receipt of any event id, not only on dispatch.
- **Distinct parse-error styling.** The status bar renders the cumulative
  SSE parse-error counter with the same style as the benign "new events" badge.
- **Column-width-aware cell truncation.** Row builders truncate to magic
  constants (20/16) that don't track the dynamically computed column widths.

## Packaging
- **Tag `pkg/logshipper`.** This repo currently depends on logshipper via a
  pseudo-version of `github.com/lasseh/taillight`'s default branch. Cut a semver
  tag (`pkg/logshipper/v0.1.0`) in the main repo and pin to it for reproducible,
  proxy-cacheable builds.
- **Configurable brand.** `component.TabBar` hardcodes the `[Taillight]` logo;
  make it a field so the component is reusable.
