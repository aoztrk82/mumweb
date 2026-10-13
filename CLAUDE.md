# CLAUDE.md

Guidance for Claude Code (and other contributors) working in this repository. Keep this file accurate: it is
loaded at the start of every session.

## What this project is

mumweb is a browser client for [Mumble](https://www.mumble.info/) voice chat. A Go **gateway** opens one normal
Mumble TCP connection per browser session and bridges it to the browser over a WebSocket; a Svelte **SPA** does
the UI and encodes/decodes Opus in the browser (WebCodecs). Public overview: [README.md](README.md).

```
Browser (Svelte SPA, WebCodecs Opus) ⇄ WSS ⇄ gateway (Go) ⇄ TLS/TCP ⇄ Mumble server
```

## Commands

| Task | Command |
|---|---|
| All tests (Go + Vitest) | `make test` |
| Go tests only | `cd gateway && go test ./...` |
| Go integration tests (need a Mumble server, e.g. `make dev`) | `cd gateway && MUMWEB_TEST_MUMBLE_ADDR=localhost:64738 go test ./internal/bridge` |
| Web tests / type check | `cd web && npm test` / `npm run check` |
| Build gateway + probe (UI embedded) | `make gateway` → `bin/mumweb`, `bin/mumprobe` |
| Production image (local) | `make docker` |
| Publish to Docker Hub (amd64+arm64) | `make publish VERSION=x.y.z` (see [docs/BUILDING.md](docs/BUILDING.md)) |
| Dev stack (Mumble + gateway, `http://localhost:8080`) | `cp deploy/.env.example deploy/.env && make dev` |
| UI with hot reload | `cd web && npm run dev` (needs the dev stack) |
| Regenerate protobuf code | `make proto` (needs `protoc` + `protoc-gen-go`) |
| Local HTTPS environment | [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) (`deploy/local/`) |

Before finishing a change: `gofmt -l gateway` is empty, `go vet ./...`, `make test`, and `npm run check` pass.
Toolchain: Go 1.27+, Node 24 LTS, Docker + Compose.

## Layout

| Path | Contents |
|---|---|
| `gateway/cmd/mumweb` | HTTP/WebSocket gateway; serves the embedded UI, `/ws`, `/healthz`; reads `MUMWEB_*` env vars |
| `gateway/cmd/mumprobe` | CLI that talks to a Mumble server directly (debugging, audio path tests) |
| `gateway/internal/mumble` | Mumble TCP client library: TLS, framing, state merging, UDPTunnel audio |
| `gateway/internal/bridge` | WS session ↔ Mumble client; JSON + binary audio protocol; limits |
| `gateway/internal/certs` | Client certificate generation |
| `gateway/proto` | Official `Mumble.proto` / `MumbleUDP.proto` + generated Go (do not hand-edit `*.pb.go`) |
| `web/src/lib` | WS client (`client.ts`), protocol (`protocol.ts`), audio pipeline (`audio/`), stores (`stores/`) |
| `web/src/components` | Svelte 5 components (runes) |
| `web/src/i18n` | UI strings: `tr.ts` (defines the shape), `en.ts`, `index.svelte.ts` (reactive `t`, language detection) |
| `deploy/` | Production compose (pulls `aoztrk82/mumweb`) + `docker-compose.build.yml` override, Caddyfile, dev compose, `local/` HTTPS test environment |
| `docs/` | English docs; Turkish mirror in `docs/tr/` |

## Conventions

- **Language:** English is primary for code, comments, commit messages, docs and deploy files. Turkish is an
  additional language: `README.tr.md` and `docs/tr/`. The web UI has English and Turkish dictionaries (`web/src/i18n/`); every user-visible string goes through `t` and must exist in both.
- **Commits:** Conventional Commits (`feat:`, `fix(gateway):`, `docs:`, `style:`…), small and focused, imperative
  subject.
- **Go:** `gofmt`, standard library first, no new dependency without a reason. Never log passwords, certificate
  keys, tokens or chat content. Every new limit or env var is documented in the README configuration table.
- **Web:** Svelte 5 runes, TypeScript strict. Any HTML from Mumble (chat, comments) goes through
  `lib/sanitize.ts` (DOMPurify); outgoing text is HTML-escaped.
- **Security posture:** the gateway only ever connects to the one configured Mumble address; browsers cannot choose
  a target. Keep Origin checks, per-IP and per-session limits intact. SuperUser web login stays off by default
  (`MUMWEB_ALLOW_SUPERUSER`).

## Protocol and server gotchas (learned the hard way; details in [docs/PROGRESS.md](docs/PROGRESS.md))

- **`frame_number` counts 10 ms units**: a 20 ms Opus packet advances it by 2, not 1.
- **Autoban:** all web users share the gateway's IP, so the Mumble server needs
  `autobanSuccessfulConnections=false` (compose files set it).
- **Bans:** Mumble's ban also bans the IP. The web "Ban" only bans the certificate (entry address `100::/128`).
- **Same-IP name reuse:** the server drops the old session when an unregistered user reuses a name from the same
  IP, so the gateway rejects a name already used by a session with a different certificate.
- **Message rate limit:** the server allows ~1 message/s (burst 5) per user and silently drops the rest; ACL writes
  cost 2 messages.
- **Channel passwords** are ACLs plus persistent access tokens, not a Mumble feature of their own.
- SuperUser cannot speak; the Mumble server image currently reports 1.5.915 while the proto is from v1.5.939.

## Keep the documentation up to date (required)

Documentation is part of every change, not a follow-up. When you change behaviour, update the matching docs in the
same commit, in **both languages** (English first, then the Turkish mirror):

| If you change… | Update |
|---|---|
| a feature, env var, port, default or deployment step | `README.md` + `README.tr.md` (feature table, configuration tables, deployment sections), `deploy/.env.example`. The compose example in the README quick start is a copy of `deploy/docker-compose.yml`: keep them identical |
| build, test or release procedure, repository layout | `docs/BUILDING.md` + `docs/tr/BUILDING.md` |
| env vars, tags, quick start (anything the Docker Hub page states) | `docs/DOCKERHUB.md`, then `make hub-readme` |
| local dev / HTTPS setup | `docs/DEVELOPMENT.md` + `docs/tr/DEVELOPMENT.md` |
| anything verified or discovered (a finding, a protocol detail, a verification result) | append to `docs/PROGRESS.md` + `docs/tr/PROGRESS.md` (newest last, before "Open issues") |
| priorities, known limits, missing features | `docs/TODO.md` + `docs/tr/TODO.md` (tick items, add new ones) |
| manual acceptance steps | `docs/TESTING.md` + `docs/tr/TESTING.md` |
| layout, commands, conventions, gotchas | this file |

Also: keep the "English | Türkçe" link at the top of each page, keep cross-links working (README documentation
table), and keep the two languages equivalent in content. If you cannot translate something properly, say so in the
Turkish file rather than leaving it silently out of date. Record decisions (what was chosen and why), not just
results.
