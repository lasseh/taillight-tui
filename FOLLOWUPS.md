# Follow-ups

Remaining deferred work. The original extraction follow-up list was executed in
the 2026-07-04 review round (see `.scratch/arch-review/REPORT.md` — 4 lenses,
refute-by-default fact-check, 58 claims / 0 refuted) on branch
`chore/arch-review-execution`: generic SSE drain (`client.Drain[T]`), netlog
view package collapsed into the shared `logview` instantiation, shared
`highlight.AttrsInline`, width-aware `highlight.Truncate`/`PadRight`
(rune/ANSI-safe), dashboard failure surfacing, SSE no-space field parsing +
hermetic parser tests, distinct parse-error styling, statusbar filter pills,
golden-fixture contract tests, plus seven functional bug fixes. The
"configurable TabBar brand" item was dropped as speculative generality; the
"generic feed client" item was obsoleted by dead-code removal.

## Remaining

- **Column-width-aware cell truncation.** Row builders truncate cells to magic
  constants (20/16) that don't track the dynamically computed column widths.
  Fixing this properly changes the adapter signatures so `eventToRow` receives
  the computed widths — a small design item, not mechanical (review lens 03 R11).

- **Tag `pkg/logshipper` and pin it.** This repo depends on logshipper via a
  stale pseudo-version of the main repo's default branch — now behind an
  upstream ingest-contract fix (client-side service validation + msg
  truncation, main repo commit `f14db64`). The main repo adopted tagged
  releases (its ADR-0005), so this is actionable **once the main repo's
  `chore/arch-review-execution` branch merges and pushes**: cut
  `pkg/logshipper/v0.1.0` there, then `go get` the tag here.

- **rebuildTable render cache (measure first).** The O(buffer) lexer re-render
  per 50 ms tick is *inferred* to lag on busy feeds, not measured — same
  discipline as the main repo's live-feed perf item: profile under loadgen
  before building a cache (review D1).

- **Releases: staying untagged** until outside consumers appear (review D2).
  Revisit when the repo is pushed and distributed.

- **Refresh golden fixtures on API changes.** `internal/tui/client/testdata/`
  documents the copy-from-main-repo procedure; re-copy after API contract
  changes and let `TestGoldenContract` catch drift.
