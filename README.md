# taillight-tui

Terminal UI and SSH server for **[Taillight](https://github.com/lasseh/taillight)** — a
real-time syslog / applog viewer.

This repository contains two binaries, extracted from the main Taillight repo so
the terminal client and SSH server can be versioned and released on their own:

| Binary | What it is |
| --- | --- |
| **`taillight-tui`** | A terminal UI client. Connects to a running Taillight API server over HTTP/SSE and renders live log streams, dashboards, and device summaries using the [Charmbracelet](https://charm.sh) stack (bubbletea / bubbles / lipgloss v2). |
| **`taillight-wish`** | An SSH server (built on [charm/wish](https://github.com/charmbracelet/wish)) that serves the same TUI to remote users over SSH — one bubbletea program per session, with public-key auth. |

Both are pure clients of the Taillight **HTTP/SSE API**; they hold no database or
server state of their own. The server, ingestion pipeline, and web frontend live
in the main repo: **https://github.com/lasseh/taillight**.

---

## Features

- Live tail of **srvlog** (syslog), **applog** (application logs), and **netlog**
  feeds over Server-Sent Events, with automatic reconnect and backoff.
- A **dashboard** with 24h summaries plus recent critical/error events.
- Per-feed **filtering** (host, program, severity, service, component, level …)
  pushed to the server so filters take effect on the stream.
- A scrollable **detail panel**, syntax-highlighted messages, and toast
  notifications for critical events.
- Tokyo Night theme, TrueColor over SSH.

---

## Install

### From source

```sh
git clone https://github.com/lasseh/taillight-tui
cd taillight-tui
make build            # builds ./taillight-tui and ./taillight-wish
```

### go install

```sh
go install github.com/lasseh/taillight-tui/cmd/taillight-tui@latest
go install github.com/lasseh/taillight-tui/cmd/taillight-wish@latest
```

---

## `taillight-tui` (client)

```sh
taillight-tui -s https://taillight.example.com -k tl_xxxxxxxx
taillight-tui -s https://dev.local -k tl_xxxxxxxx --insecure
taillight-tui -c ~/.config/taillight/tui.yml
```

| Flag | Description |
| --- | --- |
| `-s, --server` | Taillight API base URL (overrides config). |
| `-k, --key` | API key with the `read` scope (overrides config). |
| `-c, --config` | Path to a config file (see below). |
| `--insecure` | Skip TLS certificate verification (self-signed certs). |
| `--version` | Print the version and exit. |

### Configuration

`taillight-tui` reads `$XDG_CONFIG_HOME/taillight/tui.yml` (or
`~/.config/taillight/tui.yml`), overridable with `-c`. CLI flags win over the
file. See [`tui.example.yml`](tui.example.yml) for the full, commented schema.

```yaml
server:
  url: https://taillight.example.com
  api_key: tl_xxxxxxxx
  tls_skip_verify: false
display:
  fps: 30
  buffer_size: 10000
  batch_interval_ms: 50
  auto_scroll: true
  time_format: "15:04:05"
```

### Keybindings

| Key | Action |
| --- | --- |
| `1` `2` `3` `4` | Jump to Dashboard / Netlog / Srvlog / Applog |
| `tab` / `shift+tab` | Cycle to the next / previous tab |
| `↑`/`↓` or `k`/`j` | Move the table cursor |
| `enter` | Open the detail panel for the selected row |
| `/` or `ctrl+f` | Focus the inline search/filter |
| `f` | Open the filter popup for the active feed |
| `?` | Toggle the help overlay |
| `esc` | Close the detail panel / popup / go back |
| `q` or `ctrl+c` | Quit (releases SSE streams cleanly) |

---

## `taillight-wish` (SSH server)

Serves the TUI over SSH. Authentication is **public-key only** — password auth is
never enabled. The server fails to start if no `authorized_keys` file is present
(it never silently accepts all connections), unless you explicitly opt into demo
mode.

```sh
taillight-wish -s https://taillight.example.com -k tl_xxxxxxxx \
  --listen :2222 \
  --authorized-keys ~/.ssh/taillight_authorized_keys
```

Then connect with:

```sh
ssh -p 2222 taillight.example.com
```

| Flag | Default | Description |
| --- | --- | --- |
| `-s, --server` | *(required)* | Taillight API base URL. Connectivity is checked before the server starts. |
| `-k, --key` | | API key for the Taillight API. |
| `--listen` | `:2222` | SSH listen address. |
| `--host-key` | `.ssh/id_ed25519` | Path to the SSH host key. |
| `--authorized-keys` | `~/.ssh/taillight_authorized_keys` | Authorized public keys, one per line. |
| `--allow-any-key` | `false` | **DEMO ONLY** — accept any public key. Never use in production. |
| `--insecure` | `false` | Skip TLS verification when connecting to the Taillight API. |
| `--fps` | `30` | Render frame rate per session (1–60). |
| `--logshipper-enabled` | `true` | Ship the server's own logs to the Taillight applog ingest endpoint. |
| `--version` | | Print the version and exit. |

### Setting up public-key auth

```sh
# On each client machine:
cat ~/.ssh/id_ed25519.pub

# On the server, append each key (one per line):
echo "ssh-ed25519 AAAA... user@host" >> ~/.ssh/taillight_authorized_keys
```

### Self-logging

When `--logshipper-enabled` is set (the default), `taillight-wish` ships its own
session lifecycle, auth, and error logs to the Taillight applog ingest endpoint
(`<server>/api/v1/applog/ingest`) so they are queryable alongside everything
else. This is the only feature that uses the `logshipper` module.

### Docker

```sh
docker build -t taillight-wish .
docker run --rm -p 2222:2222 \
  -v ~/.ssh/taillight_authorized_keys:/authorized_keys:ro \
  -v ./host_key:/host_key:ro \
  taillight-wish -s https://taillight.example.com -k tl_xxxxxxxx \
  --authorized-keys /authorized_keys --host-key /host_key
```

See the [Dockerfile](Dockerfile) for details.

---

## The logshipper dependency

`taillight-wish` imports `github.com/lasseh/taillight/pkg/logshipper`, a small
slog handler that lives in the main Taillight repo. It is a public module,
fetched like any other dependency — no special configuration. It is currently
pinned to a pseudo-version of the main repo's default branch; once
`pkg/logshipper` is tagged upstream it should be pinned to that tag (tracked in
[FOLLOWUPS.md](FOLLOWUPS.md)).

---

## Development

```sh
make build        # build both binaries
make test         # go test -race ./...
make lint         # golangci-lint
make vulncheck    # govulncheck
make tidy         # verify go.mod/go.sum are tidy
make help         # list all targets
```

The integration tests in `internal/tui/client` are gated behind the
`integration` build tag and require a running Taillight API (`TAILLIGHT_URL`);
they are skipped by `make test`.

Known, intentionally-deferred improvements are tracked in
[FOLLOWUPS.md](FOLLOWUPS.md).

---

## License

[MIT](LICENSE) © Lasse Haugen
