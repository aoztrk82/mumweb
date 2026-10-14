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
- [x] `MUMWEB_ACCESS_PASSWORD` (off if empty): a password only the gateway knows, required to use the web UI,
      independent of the Mumble server password.
- [x] Checked in `hello` before connecting to Mumble, constant-time, with distinct `reject` reasons.
- [x] Brute-force protection: 1 s delay per wrong guess, 5 wrong guesses lock the IP for 10 minutes.
- [x] The login screen shows the field only when `/config` says so, with "remember in this browser".
- [ ] Not verified by hand in a browser yet (automated gateway tests and a type-checked UI only).
- [ ] Evaluate an invite link (signed, expiring token) instead of or alongside the password.

### 3. Web admin panel (built 2026-10-14)
- [x] Scope: online users (kick, certificate ban, mute/deafen, register), channel tree (create, rename, remove,
      channel password), full ACL and group editor, ban list (remove any entry, add certificate bans), registered
      users (list, rename, delete). Server settings (welcome text, bandwidth, user limit) are not in the protocol;
      they need Ice/gRPC or config + restart and stay out of scope.
- [x] Authentication: `MUMWEB_ADMIN_PASSWORD` (panel is off when empty) plus a Mumble account on the login form
      (SuperUser works there although `MUMWEB_ALLOW_SUPERUSER` is off for the main UI); own lockout tracker.
- [x] A separate page (`/admin/`) with its own identity (certificate key `cert-admin`), no audio and no microphone,
      so the voice client and the panel run side by side in one browser; every state-changing action is logged.
- [x] Channel password and ACL editing in the panel.
- [x] Verified against a real Mumble 1.5.915 server (Go integration test) and in headless Chrome against the local
      deployment (see PROGRESS).
- [ ] Not verified by hand by the owner yet; check the layout on a phone.
- [ ] Ban list writes replace the whole list, so a stale panel could undo a ban added elsewhere while its Bans tab was
      open (the tab re-reads on every open and after each save). Robust fix: send adds/removes by key and let the
      gateway apply them to a freshly fetched list, like `completeBans`.
- [ ] Ideas: move users between channels from the panel, a persistent audit log file, mapping an admin group to
      panel access (instead of one shared admin password), a "guard account" for desktop IP bans (see C).
- [x] Released as `0.2.0`; Docker Hub page refreshed.

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
- [x] Two-way conversation between desktop Mumble and web with a **real microphone** (owner, 2026-10-13: PC ↔ Android
      phone, voice and chat both ways).
- [ ] The desktop speaking indicator going out immediately when push-to-talk is released.
- [ ] Android, remaining checks (see the 2026-10-13 findings in D): "Add to home screen" and full-screen launch over a
      **trusted** certificate (the phone did not trust the local mkcert CA, so the install option never appeared),
      push-to-talk on the phone, Chrome for Android (the test used Samsung Internet).
- [ ] Echo without headphones (browser AEC).
- [ ] Firefox (Windows) with a real microphone; Safari 26+ (iOS/macOS): per the WebKit source WebCodecs Opus exists,
      untested on a device.
- [ ] The 17-item manual acceptance list in [TESTING.md](TESTING.md) (items 13–17: admin panel).

## C. Known limits and deferred decisions
- Found by the owner on Android (2026-10-13, Samsung Internet): voice and chat work, the responsive layout is good, and
  the session survives a locked screen. Two problems remain, both tracked in D: Firefox for Android 157.0.1 reports that
  the codec is not supported, and the session drops out of the room 15–20 s after the browser goes to the background.
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
- [ ] **Background drop on Android:** switching to another app makes the user leave the room after 15–20 s (a locked
      screen is fine). Find out whether the tab is frozen (the 20 s ping stops, the gateway then closes the session after
      60 s without a message) or the socket is killed, and test with the installed PWA, Chrome for Android and a
      foreground/media session; consider a shorter ping while hidden, reconnecting quickly on `visibilitychange`, and
      telling the user. Hypotheses only, nothing measured yet.
- [ ] **Firefox for Android 157.0.1:** "codec not supported" on login. Check which of `AudioEncoder` / `AudioDecoder`
      or the Opus configuration is missing there (the check is `encoderSupported()` / the receiver start-up), show a
      precise message, and document the result in the browser table. Firefox desktop is supported.
- [ ] "Add to home screen" test over a real certificate (e.g. a Let's Encrypt domain or a tunnel); mkcert is not
      trusted on the phone unless its root CA is installed (docs/DEVELOPMENT.md section 10).
- [ ] Pasting images into the chat (Mumble `data:` URI images; `ImageMessageLength` limit).
- [ ] Invite link: channel + suggested nickname (and a token if the A2 access password is on).
- [ ] Advanced audio settings: bitrate, jitter buffer, noise suppression options; per-listener volume for channel listeners.
- [ ] Local volume for channel listeners (`listening_volume_adjustment`).

## E. Technical debt
- [ ] Browser end-to-end test scripts (currently in the developer's temp folder, puppeteer-core + browserless): move
      them into the repo (Playwright or similar) and run them in CI.
- [ ] CI: `go vet` / `go test`, `npm run check` / `npm test`, Docker build; integration tests with a dev Mumble.
- [x] Docker Hub image `aoztrk82/mumweb` (amd64 + arm64), `make publish`, compose pulls it.
- [ ] Versioning: the gateway `release` constant (`mumweb 0.1`), git tags and a CHANGELOG; automate the image release
      in CI (tag → build → push).
- [x] Docker Hub page: description and README (`docs/DOCKERHUB.md`, `make hub-readme`).
- [ ] Test the arm64 image on real hardware; run the new README quick start end to end on a clean server.
