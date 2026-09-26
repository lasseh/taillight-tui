# Golden wire-contract fixtures

Source of truth: the main repo's generated fixtures at
`github.com/lasseh/taillight` → `api/internal/handler/testdata/golden/`
(copied verbatim from commit `e0f23e5`; re-checked byte-identical against
main-repo `main` at `02e754b` on 2026-09-26). They are generated from the real
server types by `TestGolden` there (`go test ./internal/handler -run
TestGolden -update`) and cannot be imported or embedded here because they
live under another module's `internal/` tree — a manual copy is the
consumption path.

Refresh procedure after an intentional contract change upstream:

 1. In the main repo, regenerate and commit the fixtures
    (`go test ./internal/handler -run TestGolden -update`).
 2. Copy the files listed below over this directory verbatim and update
    the commit hash above.
 3. Run `go test ./internal/tui/client` — `TestGoldenContract` fails on
    any field the client declares that the server no longer serves
    (phantom) or any new server field the client doesn't know about
    (extend the struct or add it to the fixture's skip-list).

Files copied (only the shapes this client consumes): `srvlog_event*.json`,
`netlog_event*.json`, `applog_event*.json`, `list_envelope*.json`,
`detail_envelope.json`. The ingest fixtures (`applog_ingest_*.json`) are
not copied — the TUI has no ingest surface.
