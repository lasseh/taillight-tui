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

- **Tag `pkg/logshipper` and pin it.** This repo tracks logshipper via a
  pseudo-version of the main repo's `main` (`02e754b`, includes the
  `f14db64` ingest-contract fix). The main repo's ADR-0005 tags root `v*`
  releases, which don't version the `pkg/logshipper` submodule; cut a
  `pkg/logshipper/v*` tag there, then `go get` the tag here.

- **rebuildTable render cache (measure first).** The O(buffer) lexer re-render
  per 50 ms tick is *inferred* to lag on busy feeds, not measured — same
  discipline as the main repo's live-feed perf item: profile under loadgen
  before building a cache (review D1).

- **Releases: staying untagged** until outside consumers appear (review D2).
  Revisit when the repo is pushed and distributed.

- **Refresh golden fixtures on API changes.** `internal/tui/client/testdata/`
  documents the copy-from-main-repo procedure; re-copy after API contract
  changes and let `TestGoldenContract` catch drift.
