# TODO

**English** | [Türkçe](tr/TODO.md)

Last updated: 2026-10-13. Detailed history and verification notes: [PROGRESS.md](PROGRESS.md).

## A. Priorities (in the order set by the owner)

### 1. English as the project's primary language
- [x] `README.md`, `docs/DEVELOPMENT.md`, `docs/TESTING.md`, `docs/PROGRESS.md`, `docs/TODO.md` are English; Turkish
      versions are `README.tr.md` and `docs/tr/*`, each page linking to its counterpart.
- [x] Turkish values in the deploy files (compose welcome text default, `.env.example`, `certs` service error).
- [x] Commit messages and code comments are English (checked; the only Turkish left in code is user interface text).
- [x] **Interface language.** `web/src/i18n/en.ts` next to `tr.ts`; the language follows the browser (Turkish when
      it is neither English nor Turkish) and an EN / TR switch sits on the login screen and in the status bar.
      The static `index.html` / `manifest.webmanifest` text is English.
- [ ] Decide whether the fallback for other browser languages should be English instead of Turkish
      (`FALLBACK` in `web/src/i18n/index.svelte.ts`), and whether to add more languages.
- [ ] If `MUMWEB_PLAN.md` is added to the repo, translate it to English and correct `frame_number` to "+2 / 20 ms".
- [ ] README polish before the public announcement: screenshots / a short demo GIF of the web UI, a demo or
      feature video, release badge, CI badge, a "Why mumweb?" comparison with other Mumble web clients.

### 2. Web access password (separate from the server password)
- [ ] `MUMWEB_ACCESS_PASSWORD` (off if empty): a password only the gateway knows, required to use the web UI. It is
      independent of the Mumble server password and can be turned on and off like `MUMWEB_ALLOW_SUPERUSER`.
- [ ] The gateway checks it in `hello` (before connecting to Mumble) with a constant-time comparison; a wrong one
      gets a clear `reject` message.
- [ ] Brute-force protection: per-IP attempt limit and delay (in addition to the existing connection limit).
- [ ] The login screen shows the "Access password" field only when the feature is on (the gateway can report this
      through a `/config` endpoint). Optional "remember in this browser".
- [ ] Evaluate an invite link (signed, expiring token) instead of or alongside the password.

### 3. Web admin panel (if feasible)
- [ ] Decide the scope. What the Mumble protocol allows: ACLs and groups, ban list (view/add/remove), registered
      users (UserList: list/rename/delete), channel management (exists), user management (exists). Server settings
      (welcome text, bandwidth, user limit) are not in the protocol; they need Ice/gRPC or config + restart.
- [ ] Authentication: SuperUser or a registered user in an admin group; in production a separate admin password and
      HTTPS only.
- [ ] A separate page (`/admin`), independent of the main UI; all actions logged.
- [ ] The ACL password field and channel password that even the desktop client lacks already exist and should be
      shown in the panel too.

### 4. Detailed security review (REQUIRED before production)
Goal: no input path may give access outside the container or to the host system.
- [ ] Threat model: anonymous internet user, malicious web user, malicious Mumble user (a harmful Mumble message to
      the gateway), stolen browser data.
- [ ] Container hardening: `read_only: true`, `cap_drop: [ALL]`, `security_opt: no-new-privileges`, tmpfs, resource
      limits (`mem_limit`, `pids_limit`, `cpus`), default seccomp profile, non-root user (exists), pinned image
      versions (version + digest instead of `latest`), a distroless/scratch runtime image if possible.
- [ ] Network: the gateway should only be able to reach Mumble (separate internal network, no egress); the Mumble
      admin (Ice) port closed.
- [ ] Dependency scanning: `govulncheck`, `npm audit`, image scanning (Trivy/Grype); an update policy.
- [ ] Static analysis: `gosec`, `staticcheck`, Semgrep; ESLint security rules for the frontend.
- [ ] Gateway protocol fuzzing: WS JSON and binary frames, protobuf messages from Mumble (go-fuzz / `testing.F`).
- [ ] DoS: review message size/rate limits (exist), slow clients (slowloris) and memory use, per-session queue
      limits, audio packet floods.
- [ ] Web: Content-Security-Policy, HSTS, `X-Frame-Options` / `frame-ancestors`, Referrer-Policy,
      Permissions-Policy (Caddy or the gateway); XSS tests for chat/description HTML (DOMPurify).
- [ ] The private key stored in the browser (IndexedDB) and saved channel passwords (localStorage) can be stolen if
      XSS happens; evaluate a non-extractable WebCrypto key or encryption.
- [ ] Audit the logs for secrets (passwords, keys, tokens, chat).
- [ ] TLS: Caddy settings, a recommendation to pin the fingerprint for gateway ↔ Mumble.
- [ ] Known protocol risks: IP bans made from the desktop (see C), using SuperUser from the web (off), the Mumble
      message rate limit.
- [ ] Findings go to `docs/SECURITY.md`; a vulnerability reporting path (a SECURITY policy).

### 5. Capacity testing and user limit
- [ ] A load-test tool (in the repo): N browser-like sessions, a varying number of speakers (1, 2, 5, all), duration;
      measure gateway and Mumble CPU/RAM/network. First measurement (10 sessions, all speaking): gateway ~45 % of one
      core, ~15 MiB.
- [ ] A table for target VPS sizes (e.g. 1 vCPU/1 GB, 2 vCPU/2 GB) and a recommended `MUMWEB_MAX_SESSIONS`.
- [ ] Optimise if needed: per-audio-frame `context.WithTimeout` + WS write, binary control messages instead of JSON,
      a buffer pool.
- [ ] UI: when the server is full (the gateway rejects the WS with 503) the client goes into a reconnect loop; show a
      clear message such as "Web client is full (N/N)" and stop the loop. The gateway can report capacity through
      `/config` / `/healthz`.
- [ ] Consistency with the Mumble server's own user limit (`MUMBLE_CONFIG_USERS`).

## B. Pending manual tests (owner)
- [ ] Two-way conversation between desktop Mumble and web with a **real microphone** (the tests used a synthetic tone).
- [ ] The desktop speaking indicator going out immediately when push-to-talk is released.
- [ ] Android Chrome: add to home screen → full screen, push-to-talk, behaviour when the screen locks.
- [ ] Echo without headphones (browser AEC).
- [ ] Firefox (Windows) with a real microphone; Safari 26+ (iOS/macOS): per the WebKit source WebCodecs Opus exists,
      untested on a device.
- [ ] The 12-item manual acceptance list in [TESTING.md](TESTING.md).

## C. Known limits and deferred decisions
- A web user banned from the desktop also gets the gateway IP banned (new web logins are blocked). Decision: no
  mitigation for now (small groups). If needed, an automatic fix with a "guard account".
- A global push-to-talk key while the browser is not focused is impossible (browser limit); voice activation or the
  desktop client is recommended while gaming.
- The Mumble server rate-limits messages per user (1/s, burst 5) and silently drops the excess.
- SuperUser cannot speak in Mumble (by design).
- A WASM Opus fallback (old Safari/iOS) will not be built.
- The production setup (the owner's existing Caddy + mumble-server) comes once the application is finished; that
  server will need `autobanSuccessfulConnections=false`.

## D. Missing features (compared with the desktop client)
- [ ] Editing your own comment; showing/uploading avatars.
- [ ] Viewing the channel description (editing exists).
- [ ] Whisper/shout (VoiceTarget).
- [ ] Registered user list, ban list management, ACL/group editor (see A3).
- [ ] Importing `.p12` certificates from the desktop (currently PEM only).
- [ ] Browser notifications (messages/joins while the tab is in the background), unread count in the tab title.
- [ ] Screen Wake Lock on mobile (to reduce audio cut-outs when the screen turns off).
- [ ] Pasting images into the chat (Mumble `data:` URI images; `ImageMessageLength` limit).
- [ ] Invite link: channel + suggested nickname (and a token if the A2 access password is on).
- [ ] Advanced audio settings: bitrate, jitter buffer, noise suppression options; per-listener volume for channel listeners.
- [ ] Local volume for channel listeners (`listening_volume_adjustment`).

## E. Technical debt
- [ ] Browser end-to-end test scripts (currently in the developer's temp folder, puppeteer-core + browserless): move
      them into the repo (Playwright or similar) and run them in CI.
- [ ] CI: `go vet` / `go test`, `npm run check` / `npm test`, Docker build; integration tests with a dev Mumble.
- [ ] Versioning: the gateway `release` constant (`mumweb 0.1`), tags and a CHANGELOG.
