# Security review and posture

**English** | [Türkçe](tr/SECURITY.md)

Goal: no input path may give access outside the container or to the host system, and one web user must not be able
to harm another. This page records the threat model, what is in place, what was found and decided, what is accepted,
and how to repeat the checks. First review: 2026-10-14 (release 0.2.0 plus the hardening of that day). Reporting a
vulnerability: [SECURITY.md](../SECURITY.md).

## 1. Threat model

| Actor | Can | Must not be able to |
|---|---|---|
| Anonymous internet user | open the page, open a WebSocket, send hello | reach any Mumble server but the configured one; guess passwords without being slowed and locked out; exhaust the gateway |
| Malicious web user (logged in) | send any JSON / binary frame, spam commands, pick a name, send chat HTML | impersonate or kick another web user, take another name, send audio or admin commands they should not, run script in other users' browsers |
| Malicious Mumble user | send chat, comments, channel descriptions (HTML), huge or odd values | run script in a web user's browser, crash the gateway |
| Malicious or compromised Mumble server | send any protocol message to the gateway | execute code in the gateway (the server is a configured, trusted peer, but parsing must not panic) |
| Stolen browser data | read IndexedDB and localStorage | (cannot be prevented, see 4.7: the identity key is exposed) |
| Someone with the admin password | use `/admin` | go beyond what the chosen Mumble account may do |

Trust boundaries: browser ⇄ (TLS, Caddy) ⇄ gateway ⇄ (TLS) ⇄ Mumble server. The gateway holds no secrets on disk and
stores no certificates; it sees a user's private key in memory while that user's session is open (see 4.7).

## 2. What is in place

| Area | Control |
|---|---|
| Target | one fixed `MUMWEB_MUMBLE_ADDR`; the browser cannot choose an address (no SSRF) |
| WebSocket | Origin allow-list (`MUMWEB_ALLOWED_ORIGINS`), 16 KiB JSON limit (512 KiB on admin sessions), 4 KiB audio frames, hello within 10 s, 60 s idle timeout, 10 s write timeout |
| Rates and limits | per-IP new connections (`MUMWEB_CONN_RATE`), **per-IP concurrent sessions** (`MUMWEB_MAX_SESSIONS_PER_IP`, default 10), total sessions (`MUMWEB_MAX_SESSIONS`), optional **reserved slots** for a second password (`MUMWEB_RESERVED_SLOTS`; wrong guesses delayed and locked out like the access password), audio 100 packets/s (burst 150), **control messages 50/s (burst 200), 500 dropped in a row ends the session**, admin commands paced to 0.8/s |
| HTTP server | header timeout 10 s, idle timeout 120 s, header size limit 16 KiB, non-root user |
| Passwords | access and admin password compared in constant time after hashing; 1 s delay per wrong guess; 5 wrong guesses lock the IP for 10 minutes; separate counters for access and admin; never logged |
| Names | a name already used by a session with another certificate is refused (`UsernameInUse`), so nobody can kick another web user by reusing their name |
| SuperUser | refused on the main login by default; allowed only on admin sessions that passed the admin password |
| Admin panel | off without `MUMWEB_ADMIN_PASSWORD`; admin commands refused on normal sessions; new bans from the panel must be certificate-only; SuperUser registration cannot be changed; sizes and permission masks bounded; every state-changing action audited (ids, never secrets) |
| Browser | **Content-Security-Policy** (scripts only from the gateway: no inline script, no eval; no framing; no `<object>`, `<base>`), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Permissions-Policy` (microphone for this origin only), COOP `same-origin`; **HSTS** from Caddy |
| Chat and comments | all Mumble HTML goes through DOMPurify with a tag/attribute allow-list, only `http(s)`/`mailto` links (`rel=noopener noreferrer nofollow`), only inline `data:image` pictures; typed text is escaped; 21 XSS payloads are unit-tested |
| Container | `read_only`, `cap_drop: ALL`, `no-new-privileges`, 256 MiB, 200 processes, 1 CPU, uid 100; the gateway sits on an `internal` network with Mumble and Caddy only, so it has **no route to the internet** (checked: `wget` from the container fails) |
| Mumble | Ice admin interface listens on 127.0.0.1 inside its container and is not published; autoban configured for the shared gateway IP |

## 3. Review log (2026-10-14)

| Check | Result |
|---|---|
| `govulncheck ./...` (Go 1.27.2) | no vulnerabilities |
| `npm audit` (also `--omit=dev`) | 0 vulnerabilities |
| Trivy on `aoztrk82/mumweb:0.3.0` (HIGH, CRITICAL) | 0 in the Alpine layer, 0 in the Go binary |
| `staticcheck ./...` | 1 note in a test (`TestIPRateLimit` calls `allow` three times on purpose): ignored |
| `gosec` | 3 findings, all accepted: SHA-1 in `certHash` (Mumble defines certificate hashes as SHA-1 and users are keyed by it; not a security boundary), same import flagged again (G505), and `os.ReadFile(path)` in the `mumprobe` debugging CLI (the path is the operator's own argument) |
| Fuzzing (`testing.F`, 25 s each) | `FuzzParseAudioFrame`, `FuzzClientMessage` (JSON plus the admin validators), `FuzzReadFrame` (Mumble framing and protobuf decode), `FuzzDecodeAudio`, `FuzzStateMessages` (state merging and the ACL/ban/user-list conversions): no crash. Longer runs are worth doing before big releases: `go test ./internal/mumble -run '^$' -fuzz FuzzReadFrame -fuzztime 10m` |
| XSS | 21 payload tests of `sanitizeHtml`, plus a real-browser check: a pasted `<img onerror>` shows as text, and the CSP produced no violation in the voice client or the admin panel |
| Log audit | read every `slog` call: no password, token, key, chat text or ACL group name is logged. Attacker-supplied strings that are logged (Origin header, unknown message type) are JSON-escaped by the log handler. User names are logged (operational data, not secrets) |

## 4. Findings and decisions

1. **No security headers (fixed).** The gateway sent none. Added CSP and the others listed above (gateway) and HSTS
   (Caddy). Why not HSTS in the gateway: it may be served over plain HTTP in development.
2. **Unbounded control messages from one session (fixed).** Audio was rate limited, JSON commands were not; one
   session could burn CPU with 16 KiB messages. Added the control bucket (generous: a login asks for the
   permissions of every channel) and an end-of-session after a sustained flood.
3. **One IP could take every session slot (fixed).** The per-IP rate (10 new connections a minute) still allowed
   filling `MUMWEB_MAX_SESSIONS` in five minutes. Added `MUMWEB_MAX_SESSIONS_PER_IP` (default 10). People behind one
   NAT share the limit; raise it for big offices. Behind a proxy `MUMWEB_TRUST_PROXY=true` must be set, else every
   user looks like the proxy's address.
4. **Gateway had egress and a writable filesystem (fixed in the compose files).** Hardened as in section 2.
   Mumble itself only got `no-new-privileges` and a process limit: `cap_drop: ALL` makes its entrypoint fail (it sets
   capabilities before dropping to its own user).
5. **Slowloris.** `ReadHeaderTimeout` existed; added `IdleTimeout` and `MaxHeaderBytes`. WebSocket reads have a 60 s
   idle timeout and the hello a 10 s timeout. No `WriteTimeout` on the HTTP server on purpose: it would also cut the
   long-lived WebSocket.
6. **Slow WebSocket readers.** Each write has a 10 s timeout; a slow browser ends its own session. Mumble events are
   buffered (256) per session; a full buffer backs up to that one Mumble connection only.
7. **The private key is exposed to XSS and to the gateway.** The browser keeps the Mumble identity key in IndexedDB
   and sends it with every hello, because the gateway builds the TLS client certificate and stores nothing. A
   non-extractable WebCrypto key is therefore **not possible** with this design. Mitigations: the strict CSP (no
   inline script), DOMPurify, the XSS tests, and the fact that the key only grants that one Mumble identity. Saved
   channel passwords in localStorage are low value (they only open channels). Real fix would be to move the TLS
   client to the browser, which Mumble does not allow. **Accepted.**
8. **Compromised gateway = all keys of connected users and the passwords typed while connected.** Run it as
   described (internal network, read-only, no egress); keep the host patched.
9. **Mumble message rate limit.** The server drops excess messages silently (queries too). Not a vulnerability;
   handled in the admin panel by pacing and re-reading.
10. **IP bans from the desktop client hit every web user.** Known (TODO C). The web ban is certificate-only, and the
    admin panel can delete the IP entry.
11. **`X-Forwarded-For`.** With `MUMWEB_TRUST_PROXY=true` the right-most entry is trusted. Only set it when the
    gateway is reachable through your proxy alone (the compose files do not publish its port).
12. **`/admin` is reachable by anyone who knows the URL.** It is protected by the admin password with lockout. For
    more, restrict it in Caddy (section 5) or do not set `MUMWEB_ADMIN_PASSWORD` when you do not need the panel.

## 5. Recommendations for operators

- HTTPS only; keep `MUMWEB_ALLOWED_ORIGINS` set. Pin the image: `MUMWEB_VERSION=0.3.0` in `.env`, or a digest
  (`image: aoztrk82/mumweb@sha256:…`) and update on purpose (`docker compose pull`). Same for `mumblevoip/mumble-server`
  and `caddy`.
- Pin the Mumble certificate if the gateway reaches it over a network you do not control:
  `MUMWEB_TLS_FINGERPRINT` (`mumprobe -addr host:64738 -exit` prints it). On the compose network this is optional.
- Restrict the admin page to your own address in `Caddyfile` (replace `203.0.113.7`):

  ```caddy
  @blocked {
  	path /admin /admin/*
  	not remote_ip 203.0.113.7
  }
  respond @blocked 404
  ```

  Put these lines inside the site block, before `reverse_proxy`. The panel stays usable only from that address.
  (Tested against the local Caddy: `/` answered 200, `/admin/` and `/admin` answered 404 for other addresses.)
- Use a long random `MUMWEB_ADMIN_PASSWORD` and a different Mumble SuperUser password.
- Keep Mumble's own server password or registration rules if the server should not be open to the world.
- Update policy: run `docker compose pull && docker compose up -d` after every release; maintainers re-run the checks
  below before a release and fix HIGH/CRITICAL findings first.

## 6. Repeating the checks

```sh
cd gateway
go install golang.org/x/vuln/cmd/govulncheck@latest honnef.co/go/tools/cmd/staticcheck@latest github.com/securego/gosec/v2/cmd/gosec@latest
govulncheck ./... ; staticcheck ./... ; gosec -quiet -exclude-dir=proto ./...
go test ./... -count=1                                            # includes the limit, admin and header tests
go test ./internal/bridge -run '^$' -fuzz FuzzClientMessage -fuzztime 1m
cd ../web && npm audit && npm test                                 # includes the XSS tests
docker run --rm aquasec/trivy:latest image --severity HIGH,CRITICAL aoztrk82/mumweb:<version>
```

Build the tools with the same Go version as the project (`GOTOOLCHAIN=go1.27.2`), otherwise they cannot read the code.
If Trivy's default database mirror fails, add `--db-repository ghcr.io/aquasecurity/trivy-db:2`.

## 7. Not done yet

- Semgrep and ESLint security rules for the frontend; a CI job that runs the scans above.
- Distroless/scratch runtime image (the Alpine image only adds a shell and `wget` for the health check; a built-in
  `-healthcheck` flag would remove that).
- Image digest pinning inside the compose files (documented as a recommendation instead, so installs do not go stale).
- Read-only root filesystem for Caddy and Mumble (they need writable paths; test before enabling).
- Long fuzz runs; a protocol fuzzer against a real Mumble server.
- Capacity and abuse testing under load (TODO 5).
