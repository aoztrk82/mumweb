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

## If you are the Claude on a server (deploying, not developing)

You were asked to bring mumweb up on a server. Do not build anything and do not edit the repository: the images are
on Docker Hub (`aoztrk82/mumweb`). The current job is a **1-2 day public test deployment** whose limits are looser than
production (the owner stress-tests and security-tests it from another machine; the real production configuration comes
afterwards). Follow [docs/TEST-DEPLOYMENT.md](docs/TEST-DEPLOYMENT.md) step by step; in short:

1. `git clone https://github.com/aoztrk82/mumweb.git && cd mumweb/deploy/test && cp .env.example .env`.
2. Ask the owner for the missing `.env` values (domain whose DNS already points here, SuperUser, access, admin and
   reserved-slot passwords). Never invent or commit secrets; `.env` is git-ignored.
3. Ports 80/tcp, 443/tcp+udp and 64738/tcp+udp must be open. `docker compose pull && docker compose up -d`.
4. Run the checks in that page (`/healthz`, `/config`, headers, read-only/no-egress gateway) and report the results and
   `docker compose ps` to the owner.

Rules: keep the hardening in the compose file (read-only gateway, dropped capabilities, internal network) even if
something seems easier without it, and report problems instead; never publish port 8080; many connections and failed
logins from the owner's test machine are expected. Update with `git pull && docker compose pull && docker compose up -d`.
The compose in `deploy/test/` is for the test only; `deploy/docker-compose.yml` is the production one.

## Commands

| Task | Command |
|---|---|
| All tests (Go + Vitest) | `make test` |
| Go tests only | `cd gateway && go test ./...` |
| Go integration tests (need a Mumble server, e.g. `make dev`) | `cd gateway && MUMWEB_TEST_MUMBLE_ADDR=localhost:64738 MUMWEB_TEST_SUPERUSER_PASSWORD=<pw> go test ./internal/bridge` (the password enables the SuperUser/admin tests) |
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
| `gateway/internal/bridge` | WS session ↔ Mumble client; JSON + binary audio protocol; limits; `admin.go` = admin-panel role (password, validation, pacing, audit log) |
| `gateway/internal/certs` | Client certificate generation |
| `gateway/proto` | Official `Mumble.proto` / `MumbleUDP.proto` + generated Go (do not hand-edit `*.pb.go`) |
| `web/src/lib` | WS client (`client.ts`), protocol (`protocol.ts`), audio pipeline (`audio/`), stores (`stores/`) |
| `web/src/components` | Svelte 5 components (runes) |
| `web/src/admin`, `web/admin` | Admin panel: second Vite page at `/admin/` (own client + store, no audio; ACL/ban/user-list editors) |
| `web/src/i18n` | UI strings: `tr.ts` (defines the shape), `en.ts`, `ar.ts`, `de.ts`, `es.ts`, `it.ts`, `ru.ts`, `index.svelte.ts` (reactive `t`, language detection, RTL for Arabic) |
| `deploy/` | Production compose (pulls `aoztrk82/mumweb`) + `docker-compose.build.yml` override, Caddyfile, dev compose, `local/` HTTPS test environment, `test/` loose-limit compose for the 1-2 day server test |
| `docs/` | English docs; Turkish mirror in `docs/tr/` |
| `CONTRIBUTING.md` / `CONTRIBUTING.tr.md` | How to contribute; says explicitly that this project is AI-coded and that AI-assisted PRs are welcome, judged like any other |

## Conventions

- **Language:** English is primary for code, comments, commit messages, docs and deploy files. Turkish is an
  additional language: `README.tr.md` and `docs/tr/`. The web UI has seven dictionaries — `tr.ts` (defines the
  shape), `en.ts`, `ar.ts`, `de.ts`, `es.ts`, `it.ts`, `ru.ts` (`web/src/i18n/`); every user-visible string goes
  through `t` and must exist in all of them (`npm run check` catches a missing key as a type error). Arabic is the
  only RTL language: `apply()` in `index.svelte.ts` flips `<html dir>`; there is no other RTL-specific styling, so a
  new component should avoid hardcoding `left`/`right` and prefer flexbox/logical properties where it matters.
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
- **Admin panel (`/admin/`):** a bridge session with `hello.role = "admin"`; off unless `MUMWEB_ADMIN_PASSWORD` is set. It
  has its own lockout tracker, certificate key (`cert-admin`) and no audio, so voice + admin can run in one browser, but
  they must be **different Mumble accounts** (the server drops the older session of the same registered account).
  SuperUser is allowed only there. Never log ACL group names (`#name` = channel password).
- **Mumble's rate limit also drops queries.** Admin commands are paced in the gateway; the panel re-reads a list after a
  write instead of trusting its draft. New ban entries from the panel must be certificate-only (`100::/128` + hash).
- **Security headers/CSP** live in `securityHeaders` (`gateway/cmd/mumweb`): no inline script or eval in the UI (`script-src 'self'`). Check the browser console for CSP errors when adding a script, worker or remote resource. HSTS is set in the Caddyfiles, not the gateway.
- **Compose hardening** (gateway read-only, no caps, internal network without egress): do not add gateway features that write to disk or call out. Mumble must not get `cap_drop: ALL` (its entrypoint breaks).
- **Reserved web slots** (`MUMWEB_RESERVED_SLOTS` + `_PASSWORD`): `acquireSlot` in `bridge.go` runs after the login checks; the hard `MaxSessions` check stays at socket accept. Admin sessions count as privileged. Reject reasons `WebFull`/`ReservedPassword*` must exist in both i18n dictionaries.
- **Limits:** per-IP sessions, control-message bucket (50/s, burst 200: login queries permissions of every channel) and audio bucket; keep them when changing `readLoop`. Security results and the operator guide: [docs/SECURITY.md](docs/SECURITY.md) (re-run the scans listed there before a release).
- **`sw.js` must never cache `/admin*`** and may only store `/` as the app shell (bump `CACHE` when its logic changes).
- Windows checkout has CRLF files: check Go formatting with `tr -d '\r' < f | gofmt -l`, not `gofmt -l` on the CRLF file.

## Keep the documentation up to date (required)

Documentation is part of every change, not a follow-up. When you change behaviour, update the matching docs in the
same commit, in **both languages** (English first, then the Turkish mirror):

| If you change… | Update |
|---|---|
| a feature, env var, port, default or deployment step | `README.md` + `README.tr.md` (feature table, configuration tables, deployment sections), `deploy/.env.example`. The compose example in the README quick start is a copy of `deploy/docker-compose.yml`: keep them identical |
| build, test or release procedure, repository layout | `docs/BUILDING.md` + `docs/tr/BUILDING.md` |
| env vars, tags, quick start (anything the Docker Hub page states) | `docs/DOCKERHUB.md`, then `make hub-readme` |
| local dev / HTTPS setup | `docs/DEVELOPMENT.md` + `docs/tr/DEVELOPMENT.md` |
| the server test deployment (`deploy/test/`) | `docs/TEST-DEPLOYMENT.md` + `docs/tr/TEST-DEPLOYMENT.md` and the "If you are the Claude on a server" section above |
| anything verified or discovered (a finding, a protocol detail, a verification result) | append to `docs/PROGRESS.md` + `docs/tr/PROGRESS.md` (newest last, before "Open issues") |
| priorities, known limits, missing features | `docs/TODO.md` + `docs/tr/TODO.md` (tick items, add new ones) |
| manual acceptance steps | `docs/TESTING.md` + `docs/tr/TESTING.md` |
| layout, commands, conventions, gotchas | this file |

Also: keep the "English | Türkçe" link at the top of each page, keep cross-links working (README documentation
table), and keep the two languages equivalent in content. If you cannot translate something properly, say so in the
Turkish file rather than leaving it silently out of date. Record decisions (what was chosen and why), not just
results.
