# Progress log

**English** | [Türkçe](tr/PROGRESS.md)

A chronological record of what was built, how it was verified, and what was found along the way. Newest work goes at
the end (before "Open issues"). See [TODO.md](TODO.md) for what is next.

## Phase 0: done (2026-10-13)

Repository skeleton; no application code yet.

### Tool versions

| Tool | Version |
|---|---|
| Go | 1.27.2 (linux/amd64) |
| Node / npm | 24.21.0 (LTS, nvm) / 11.19.0 |
| protoc | libprotoc 3.21.12 |
| protoc-gen-go | v1.36.12 |
| Docker / Compose | 29.8.1 / v5.5.1 |
| gh | 2.102.0 |
| GNU Make | 4.4.1 |

Dependencies: Go `google.golang.org/protobuf` v1.36.12, `github.com/coder/websocket` v1.8.15 (not imported yet;
`go mod tidy` run before Phase 1 would drop it from go.mod). Web: Svelte 5.57, Vite 8.3, TypeScript 6.0,
`dompurify` 3.4.17, `vitest` 5.0. `@types/dompurify` was not added: DOMPurify 3.x ships its own types and the
`@types` package is a deprecated stub.

### Mumble proto

- Source: `mumble-voip/mumble` tag **v1.5.939** (latest stable 1.5.x), `src/Mumble.proto` and `src/MumbleUDP.proto`.
- Local changes: a source comment and `option go_package` were added at the top
  (`gateway/proto/mumbleproto`, `gateway/proto/mumbleudp`). Regenerate with `make proto`.
- Note: the `mumblevoip/mumble-server:latest` image currently runs server version **1.5.915**.

### Verification

- `go build ./...` and `go vet ./...` clean.
- `npm run check`: 0 errors / 0 warnings; `npm run build` outputs to `gateway/web/dist`.
- `make gateway` → `bin/mumweb`, `bin/mumprobe`; the binary serves the embedded SPA. A "not built" placeholder page
  is returned when `dist` is empty (also tested).
- `make test`: go test (no tests) and vitest (`--passWithNoTests`) passed.
- `make dev`: both containers came up; gateway healthcheck `healthy`, `curl localhost:8080/healthz` → 200, `/` serves
  a page titled "mumweb" and its JS asset with 200, the gateway container runs as the non-root `mumweb` (uid 100)
  user, the mumble-server TCP port is reachable and the superuser password is set.
- 64738 was in use by another Mumble server locally, so verification used `MUMBLE_PORT=64739` (the host port in the
  compose file is configurable with `MUMBLE_PORT`).

## Phase 1: Mumble TCP client library + mumprobe (2026-10-13)

`gateway/internal/mumble`:
- `messages.go`: type numbers (Mumble.proto order, 0–26), `[uint16 BE type][uint32 BE length]` framing, the same size
  limit as the server (0x7fffff). Unknown types are logged at debug level and skipped.
- `conn.go`: TLS (verification off by default; `TLSVerify` and SHA-256 fingerprint pinning supported), `Version`
  (v1 + v2 = 1.5.0, release/os), `Authenticate` (opus=true), synchronous handshake up to `ServerSync`,
  `Reject` → `*RejectError`. Afterwards a read loop with a 15 s ping, 60 s read timeout and `context`
  cancellation (Close waits for all goroutines). Events are published on a buffered Go channel.
- `state.go`: channel/user maps, partial `ChannelState`/`UserState` merging (including links_add/remove), a registered
  user when a `user_id` field is present.
- `audio.go`: `MumbleUDP.Audio` ↔ tunnel payload (`0x00` header); the legacy format is skipped with `ErrLegacyAudio`.
- `events.go`: event types.

`cmd/mumprobe`: prints the channel tree and users, logs events (audio: first packet, every 50th and the
terminator). Flags: `-addr -user -password -cert -key -verify -fingerprint -say -join -tx <duration> -exit -debug`.
`-tx` sends valid 20 ms Opus silence packets (`f8 ff fe`) to exercise the audio path without a desktop client.

### Verification (dev server 1.5.915, `localhost:64739`)
- `go vet`, `go test ./...` clean: framing encode/decode and errors, partial UserState/ChannelState merging, audio
  encode/decode (target=0 oneof present on the wire, legacy detection).
- Two mumprobes: `listener` and `talker -say 'merhaba <b>dünya</b>' -tx 1s`. The listener received the talker's
  join, the text and **protobuf-format** audio packets (frame 0 … 49, `term=true` on the last) over the TCP tunnel;
  no legacy packets seen. `userRemove` arrived when the talker left.
- Wrong SuperUser password → `rejected (WrongUserPW)`; with the right password SuperUser appears as `[registered]`.
- **Pending manual check:** channel list in the desktop Mumble client, talking from the desktop, and the `-say`
  message appearing on the desktop (to be done by the user).

## Phase 2: WebSocket bridge (2026-10-13)

- `internal/certs`: RSA 2048 self-signed client certificate (CN = username, 20 years, PKCS#8 key); `Load` validates a
  PEM pair. Nothing is stored on disk.
- `internal/bridge`: `GET /ws`. One `mumble.Client` per WS session. The protocol of plan section 6.
  - Flow: `hello` (within 10 s) → generate a certificate if none + `cert` → connect to Mumble → `channel`*, `user`*
    → `ready`. (Difference from the plan order: `ready` comes **after** the initial state; when the browser gets
    `ready` the tree is complete.)
  - `user`/`channel` are always sent as full merged records; the root channel's `parent` is `null`.
  - Added fields: `ready.allowHtml`, `ready.messageLength`; `denied.type`; `text.private` (direct message); tree
    messages are added to `channelIds`.
  - Errors/closing: `reject` (WS 1000); `error` (WS 1013) when Mumble is unreachable or the connection drops;
    protocol violation 1008. Closing the WS closes the Mumble connection and vice versa; `Shutdown()` ends all sessions.
  - Limits: Origin list (library same-host check when empty), `MUMWEB_MAX_SESSIONS`, `MUMWEB_CONN_RATE` (10) new
    connections per IP per minute, JSON 16 KB, audio frame 4 KB, 100 audio packets per second per session
    (excess dropped).
  - No passwords, keys or chat content in the logs.
- `cmd/mumweb`: the environment variables of plan section 10. Additions to the plan: `MUMWEB_CONN_RATE` (default 10)
  and `MUMWEB_TRUST_PROXY` (default false; when true the client IP is the rightmost `X-Forwarded-For` value; must be
  enabled behind Caddy, otherwise the IP limit counts everyone as one IP).
- `mumble.Dial` now also waits for the `ServerConfig` that follows `ServerSync` (it used to be missing from `ready`).

### Important finding: Mumble autoban
All web users connect to Mumble from the gateway's single IP. Mumble's default (10 connections in 120 s → 300 s
global ban, **successful connections included**) banned the gateway during tests (`Ignoring connection ... (Global
ban)`). `MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS=false` was added to the dev compose. The decision for the
production compose was left to Phase 7 (see "Open issues").

### Verification
- Unit: binary frame encode/decode, root channel `parent:null`, token bucket, IP limit, Origin rejection (403),
  session limit (503), message without `hello` → 1008.
- Integration (`MUMWEB_TEST_MUMBLE_ADDR=localhost:64739 go test ./internal/bridge`, 5 repetitions): a Go WS client:
  `hello` → `cert`, `channel`, `user`, `ready`; ping/pong; text both ways; `setSelf` visible to an observer; audio
  both ways (frame number + terminator preserved); closing the WS makes the observer receive `UserRemove`; a second
  login with the same certificate yields the same certificate hash; wrong SuperUser password → `reject
  WrongUserPW`; goroutine count returns to its starting value once all sessions end.
- A Node (built-in WebSocket) client connected to the real gateway in the Docker dev stack; mumprobe as observer
  saw the user, the text and the exit.
- **Pending manual check:** the user dropping from the desktop client when the WS closes.

## Phase 3: UI skeleton (2026-10-13)

- `web/src/lib/ws.ts`: `ReconnectingSocket`: 1, 2, 4, 8 … 30 s backoff (reset after `ready`), the wait is skipped
  on the `online` event, `ping` every 20 s; if no `pong` arrives within 10 s the connection is considered dead and
  re-established. No retry after 1008 (protocol violation).
- `lib/client.ts`: actions (connect/disconnect/joinChannel/setSelf/sendText) and handling of server messages.
  `reject`, kick/ban (`userRemove` for our own session) and an invalid certificate stop reconnecting and show an
  error on the login screen. Other disconnects reconnect; chat history is kept, server state is reset on each
  connection.
- `lib/stores/session.svelte.ts` (Svelte 5 runes, `SvelteMap`), `lib/certStore.ts` (IndexedDB),
  `lib/settings.ts` (localStorage, nickname), `lib/sanitize.ts` (DOMPurify: `b i u s strong em a br p img span`;
  links only http/https/mailto and in a new tab; img only `data:image/*;base64`). Outgoing text is HTML-escaped.
- Components: `Login`, `ChannelTree` (recursive; sorted by position → name; double-click/Enter switches channel),
  `Chat` (Enter sends, Shift+Enter newline; channel join/leave lines), `Controls` (mute/deaf/exit), `StatusBar`.
  Channels/Chat tabs at ≤640 px. Dark theme by default; `data-theme="light"` ready for the light theme.
- Gateway: a `code` field on `error` messages (`invalid_certificate`, `mumble_unavailable`, `disconnected`); the
  session closes if nothing arrives from the browser for 60 s.
- mumprobe: `-mkchannel` (to create test channels) and the certificate hash in the user log.

### Verification
- `npm run check` 0 errors; Vitest: binary frame (same bytes as the Go test), backoff, `textToHtml`.
- Docker dev stack + temporary headless Chromium (browserless image on the dev network) + a puppeteer-core script
  (in the scratchpad, not added to the repo), with a mumprobe observer:
  login → user visible in Mumble, observer visible in the tree; chat both ways; `<script>`, an `onerror` img and a
  `javascript:` link have no effect; join/leave lines; double-click channel switching twice is reflected in Mumble;
  mute/deaf reflected in Mumble and icons shown; the nickname is remembered after a reload and the **certificate
  hash is the same**; logout drops the user; wrong SuperUser password → "This nickname is registered; wrong
  password or certificate."; 390 px mobile layout.
- Restarting the gateway container → the UI showed "Reconnecting (attempt 1, 1 s)" and reconnected on its own, chat
  history kept.
- Ghost session: a second connection with the same name + same certificate gets `ready`, Mumble drops the old
  session (reconnecting after a Wi-Fi drop does not hit `UsernameInUse`).
- **Pending manual check:** the browser user appearing in desktop Mumble, two-way channel switching and chat, the
  browser updating when moved from the desktop, staying a registered user (self-registration from the desktop/admin).

## Phase 4: receiving audio (2026-10-13)

- `web/src/lib/audio/context.ts`: a single `AudioContext` (48 kHz, `interactive`), opened on the login click.
- `audio/receiver.ts`: a WebCodecs `AudioDecoder` per speaker (opus, 48 kHz, mono); late/duplicate packets
  (`frameNumber` ≤ last) are dropped; once the terminator's PCM is out the worklet gets `end`; "not speaking" after
  300 ms without packets; a decoder idle for 30 s is closed; recreated on a decoder error. Gain = local volume
  (0–200 %, local mute) × `volume_adjustment` (1 if 0). A warning banner appears if `AudioDecoder` does not support
  opus (Phase 9).
- `audio/playback-worklet.ts` + `audio/jitter.ts`: a jitter buffer per user (60 ms prefill, at most 200 ms, the
  oldest is dropped on overflow, silence + re-prefill when empty, the terminator plays what is left and ends the
  stream), linear resampling if the context is not 48 kHz, all users mixed and clipped to [-1, 1].
- On deafen the worklet is cleared immediately; packets are received but not played (the speaking indicator keeps
  working).
- UI: a green ring on the speaking user's avatar; clicking another user shows a volume slider, "Mute for me" and
  "Reset" (stored in localStorage by user name).
- mumprobe: `-play <file>` (length-prefixed Opus packets, duration computed from the TOC and sent in real time).

### Protocol correction: the unit of `frame_number`
Plan 5.3 says "+1 per 20 ms frame". The official client (v1.5.939 `AudioInput.cpp`/`AudioOutputSpeech.cpp`):
`iFrameSize = sampleRate / 100` (10 ms); the sent `frameNumber = iFrameCounter - iBufferedFrames` counts 10 ms
frames, and the receiver's jitter buffer uses `timestamp = iFrameSize * frameNumber` and `span = samples in the
packet`. So for a 20 ms packet the counter must increase by **2** (with +1 the timestamps overlap in the desktop
receiver). mumprobe `-tx`/`-play` were fixed accordingly; in Phase 5 the browser increases it by `samples / 480`.

### Verification
- Vitest: jitter buffer (prefill, gain/mixing, 200 ms limit, short burst after terminator, underrun, 44.1 kHz
  resampling). `npm run check` clean.
- End to end (headless Chromium, `--unsafely-treat-insecure-origin-as-secure=http://gateway:8080`): 440 Hz and
  660 Hz (amplitude 0.3) Opus packets were produced with the browser's `AudioEncoder` on a test page, sent to
  Mumble with `mumprobe -play`, and the browser output was measured with an AnalyserNode:
  - one speaker: RMS 0.212 (= 0.3/√2, level preserved), 440 Hz at -24 dB, every other bin < -84 dB; speaking indicator 1.
  - when it ends RMS 0, indicator 0.
  - two speakers at once: 440 Hz at -24 dB and 660 Hz at -25 dB, RMS 0.301 (√2 times), both indicators together.
  - with the 660 Hz speaker locally muted 660 Hz at -112 dB, 440 Hz preserved.
  - deafened: RMS 0.
- **Pending manual check:** clean audio of real desktop speech in the browser (and 2–3 people at once).

## Phase 5: sending audio (2026-10-13)

- `audio/capture-worklet.ts` + `audio/frames.ts`: microphone → 20 ms / 960 sample 48 kHz mono frames + RMS; linear
  resampling if the context is not 48 kHz. If the stream/context sample rates differ in Firefox a separate capture
  context is opened.
- `audio/transmitter.ts`: `getUserMedia` (AEC, NS [adjustable], AGC, mono; falls back to the default device if the
  saved one is missing), `AudioEncoder` (opus, 48 kHz, mono, 20 ms, complexity 9, DTX off). Bitrate = min(40 000,
  maxBandwidth − 19 600): the TCP-mode overhead of the official `getNetworkBandwidth`. Nothing is sent while
  muted/deafened/server-muted/suppressed (the level meter keeps working).
  - **Terminator:** when speech ends the last frame is encoded too, outputs are held until `flush()` completes and
    the **last packet** is marked as the terminator. (In the first version the lookahead queue produced by flush was
    sent as a separate packet after the terminator; caught in testing and fixed.)
  - Frame number +2 per packet (10 ms unit, see the Phase 4 note).
- PTT: a configurable key (default `V`; not triggered while typing in a text field; released when the window loses
  focus) and a hold-down button on the control bar. VAD: instant open, 300 ms hold; the threshold is set with the
  slider above the level bar (-60…0 dBFS scale). Settings are in localStorage (`mumweb.audio`).
- Settings window (expanded in Phase 6): microphone choice, PTT key assignment. Without microphone permission a
  "you can only listen" warning appears and the session continues.

### Verification
- Vitest: FrameAssembler (48 kHz split, 44.1→48 kHz), VAD hold time, bitrate limit, level scale.
- End to end: a WAV as the fake microphone in Chromium (1.5 s of 1 kHz + 1.5 s silence, looped); A speaks, B (muted)
  listens and the output is measured with an FFT, mumprobe logs the packets:
  - VAD: speech blocks 0→180, 182→364 … (90 packets = 1.5 s + 300 ms), `term=true` at the end; **no packets** in
    the silent part; no stray packets after the terminator.
  - 1 kHz heard at B (-16 dB; the browser AGC boosts it by +8 dB). With browser audio processing off: RMS 0.212
    (exact) and other frequencies < -88 dB, i.e. the pipeline is clean.
  - PTT: 3 s without the key → 0 packets; 2 s held → 52 packets (506…712, steps of +2); terminator 22 ms after release.
  - Typing "vvv" in the chat sends no packets; VAD + mute → 0 packets.
  - Threshold 1.0 → 0 packets while the tone plays; 0.3 → packets flow; the setting persists.
- **Pending manual check:** speech from the browser heard clearly on the desktop and the desktop speaking indicator
  going out immediately when PTT is released; echo cancellation with a real microphone (Chrome's AEC may not use
  WebAudio output as a reference on every platform; note any echo when used without headphones).

## Phase 6: polish (2026-10-13)

- Settings screen: microphone, speaker/headphones (`AudioContext.setSinkId`; explanation in browsers that lack it),
  noise suppression (reopens the microphone), talk mode, PTT key assignment, VAD threshold (with a live level bar),
  theme (dark default / light), join/leave sounds, certificate: export (PEM: certificate + key), import (PEM), reset
  (with a warning); "Reconnect now" after changes.
  Note: the desktop Mumble's `.p12` export is not supported (convert to PEM, e.g. `openssl pkcs12 -nodes`).
- Mobile (≤ 640 px): tabbed layout, and in PTT mode a full-width 72 px push-to-talk button above the control bar
  (touch, pointer capture, long-press menu disabled); the control bar shrinks to icons, no horizontal overflow.
  Between 641 and 900 px the button labels are hidden and the bar is a single row.
- Notification sounds: a short rising two-tone when someone joins our channel and a falling one when they leave
  (synthesised with WebAudio, silent while deafened, can be turned off).
- PWA: `manifest.webmanifest` (standalone), icons (192, 512, maskable 512, apple-touch 180; generated from the
  SVG), `sw.js` caches only static files (navigation: network first; `/assets/*`: cache first; `/ws`, `/healthz`
  excluded). Gateway: `.webmanifest` MIME type, 1 year `immutable` for `/assets/*`, `no-cache` for the rest,
  `nosniff`.
- The connection status bar and the error/reject texts were done in Phase 3.

### Verification
- Vitest: PEM bundle parsing (round trip, RSA key, missing part).
- End to end (headless Chromium):
  - Chrome `Page.getInstallabilityErrors` → empty, no manifest errors, service worker active.
  - When someone joined and left the channel: 4 oscillators (2 tones × 2); 0 with "sounds" off.
  - Light theme → `data-theme="light"`, the background changes.
  - Certificate: the exported PEM contains certificate + key; reset → new identity (hash changed); import → after
    reconnecting the **original hash came back**.
  - Android Chrome emulation (390×800, touch): touching the big PTT button for 1.2 s → 66 packets + terminator.
  - At 800 px and 390 px the control bar is a single row and the page does not overflow horizontally (PTT and VAD modes).
- **Pending manual check:** on a real Android Chrome, "Add to home screen" → full-screen launch and push-to-talk
  (needs HTTPS; after the Phase 7 deployment).

## Phase 7: deployment (2026-10-13)

- `Dockerfile` (from Phase 0): web build with Node → Go build (embed) → alpine, non-root `mumweb` user,
  `HEALTHCHECK /healthz`.
- `deploy/docker-compose.yml` (per plan section 13) + `deploy/Caddyfile` + `deploy/.env.example`. Differences from
  the plan:
  - `container_name`s removed (unnecessary; they clashed with existing `mumble-server`/`caddy` containers on the same server).
  - Mumble: `MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS=false` (see the Phase 2 finding); the welcome text and
    user limit can be changed from `.env`.
  - mumweb: `MUMWEB_TRUST_PROXY=true` (real client IP behind Caddy), `MUMWEB_ALLOWED_ORIGINS=https://${MUMWEB_DOMAIN}`.
  - Caddy: `443/udp` is also open for HTTP/3.
- README: installation, DNS, the autoban explanation, using an existing reverse proxy (Caddy/nginx examples) or an
  existing Mumble server, configuration table, updating/backup, troubleshooting, development.
- mumprobe and `mumble.Client.ServerFingerprint()`: print the server certificate's SHA-256 fingerprint in the
  `MUMWEB_TLS_FINGERPRINT` format.

### Verification
- The production compose was brought up locally under a separate project name (`mumweb-prodtest`); only the ports
  were changed with an override and, with no real domain, Caddy got `tls internal` (domain `ses.test`):
  - 3 services up, mumweb `healthy`, uid 100 (`mumweb`); the autoban/welcome/user settings applied in the Mumble ini.
  - `https://ses.test/healthz` → 200 (HTTP/2). `/ws` with a wrong Origin → 403, with the right Origin → 101.
  - Headless Chromium (no flags, real HTTPS secure context): two browser sessions connected through Caddy/TLS, the
    1 kHz tone of the speaking page was heard in the listening page (-16 dB), service worker `activated`, the
    mumprobe observer saw the users and audio. The IP in the gateway log is the browser's, not Caddy's.
  - The test stack was removed with its volumes.
- Fingerprint pinning: connects with the right value, `fingerprint mismatch` with a value changed by one byte.
- **Pending manual check:** on a clean VPS with a real domain, `docker compose up -d` → `https://voice.<domain>` and
  end-to-end speech (including the Let's Encrypt certificate).

## Phase 8: tests and verification (2026-10-13)

- Go: `internal/mumble/conn_test.go` added: with a fake TLS Mumble server (no server needed) Version/Authenticate
  content, **skipping of unknown message types** before and after the handshake, state/`Info()` after `ServerSync`
  + `ServerConfig`, events, outgoing `UserState`/`UDPTunnel` frames, deaf ⇒ mute, the event stream ending with an
  error when the server closes, `Reject`, handshake timeout, fingerprint mismatch, goroutines finishing after
  `Close`. (In the first version the fingerprint test passed for the wrong reason (a timeout) because the fake
  server did not drive the TLS handshake; fixed, and the error text is now checked too.)
- Existing tests: framing, merging, audio encode/decode, certificates, bridge binary frames and limits,
  integration against a real server; 19 Vitest tests (binary frame, jitter buffer, framing/resampling, VAD,
  bitrate, PEM, backoff).
- Test commands and the section 14 manual acceptance list were added to the README.

### Resource usage (automated counterpart of manual test 12)
Dev stack, 8-core machine. 10 WebSocket clients in the same channel, **all talking continuously at once** (50
packets/s each; worst case): 496 packets/s into the gateway, 4460 packets/s out (4461/s expected, no loss).

| | Idle | 10 sessions, 10 speakers |
|---|---|---|
| gateway CPU | 0 % | 40–46 % of one core |
| gateway RAM | 4.9 MiB | ~15 MiB |
| mumble-server CPU / RAM | – | ~15 % / 6.7 MiB |

In reality 1–2 people speak at a time; the load is proportional to speakers × listeners. If needed the first
optimisation candidate is the per-audio-frame `context.WithTimeout` + WS write.

## Addendum: local HTTPS test environment (2026-10-13)

- `deploy/local/compose.yaml` (Caddy + Mumble + mumweb), `deploy/local/Caddyfile` (mkcert certificate in `certs/`,
  `auto_https disable_certs`), `.env.example` (domain, ports, SuperUser password); works without a `.env`. The
  contents of `certs/` stay out of git and the Docker build context. `.gitattributes` for LF in Windows checkouts.
- Gateway: allowed-Origin comparison normalised (lower case, trailing `/`, default `:443`/`:80` ignored); so compose
  writes `https://<domain>:<HTTPS_PORT>` and it works on 443 as well as on other ports.
- `docs/DEVELOPMENT.md`: step-by-step guide for Windows (mkcert, hosts, start, desktop client, port conflicts,
  LAN/Android, troubleshooting); linked from the README.

### Verification (Linux, a CA + certificate with the same structure made with openssl instead of mkcert)
- 80/443/64738 were taken on this machine so 8444/8082/64741 were used via `.env` (the port-change path was tested too).
- Caddy served the file certificate without going to ACME; `curl --cacert rootCA.pem
  https://mumweb.test:8444/healthz` → 200.
- Headless Chromium (`mumweb.test` → host IP): secure context, audio between two sessions (1 kHz, -16 dB), service
  worker, Origin `https://mumweb.test:8444` accepted; mumprobe saw the users and audio via `localhost:64741`.
- The environment was removed with `down -v`, the fake certificates deleted.
- **Pending:** a trial on real Windows + Docker Desktop + mkcert.

## Addendum: user feedback: context menu and microphone errors (2026-10-13)

From a local test on Windows + Firefox: a "microphone unavailable" warning and the Firefox menu opening on right-click.
- **Context menu** (`ContextMenu.svelte`): user → local volume (0–200 %), "Mute for me", "Reset"; own row →
  microphone/audio on-off; channel → "Join channel" (double-click still works). Fits within the screen edges;
  closes on Esc, outside click, scroll and window change; opens with the keyboard menu key/Shift+F10 and
  long-press on Android. The volume panel opened by left click was removed.
- **Microphone:** if the processed (AEC/NS/AGC, mono) `getUserMedia` is rejected with a non-permission error it is
  retried once with `{audio: true}` (some Windows drivers reject the processed open). The warning now explains by
  error type (Windows privacy setting, another app, no device…) and shows the technical error name; a "Try again"
  button was added.
- Verification: a temporary Firefox 156 container (Selenium): normal open with a fake microphone; forcing Firefox to a
  different sample rate also works (the user's error was not due to that); when the processed request was rejected
  with `NotReadableError` the plain request opened the microphone; when both were rejected the explanation +
  `(NotReadableError: …)` appeared and the microphone opened after "Try again". Menu: the `contextmenu` event is
  `defaultPrevented` (the Firefox menu does not open); with WebDriver right-click the user/channel/own menus work,
  50 % → badge "50%", local mute → "0%" and persistent, joining a channel from the menu is reflected in Mumble,
  mute from the menu is reflected in Mumble, Esc/outside click closes, left click does not open it.
- **Pending:** seeing the real error name on the user's Windows machine (the new warning will show it).

## Addendum: SuperUser switch, creating channels from the web, local Mumble certificate (2026-10-13)

- Gateway `MUMWEB_ALLOW_SUPERUSER` (default **false**): when off the `SuperUser` name (case insensitive) is rejected
  with `reject SuperUserDisabled` before connecting to Mumble. On in the local and dev compose, off in production.
- Protocol: `{t:"createChannel", parent, name, temporary}` → Mumble `ChannelState`. The server's ACL decides
  permission; a refusal appears in the chat as `denied`. UI: channel right-click → "Create sub-channel…" window
  (name, temporary channel). Note: plan section 1 put administration out of scope; only channel creation was added,
  at the user's request.
- In the local environment the Mumble server also uses the mkcert certificate (`sslCert`/`sslKey`). mkcert writes the
  key with mode 600 and Mumble runs as uid 10000, so the one-shot `certs` service copies the files into a volume
  with suitable ownership (a bind mount made it exit with "Failed to read /certs/key.pem" on Linux). If the
  certificate is missing it stops with an error message.
- Verification: Go tests (SuperUser refused on 3 spellings; an unregistered user creating a permanent channel →
  `Permission`; SuperUser creates a temporary channel). Local stack (openssl CA like mkcert): `openssl s_client
  -CAfile rootCA.pem -verify_hostname mumweb.test` → `Verify return code: 0` on the Mumble port. Browser: SuperUser
  logged in from the web (registered icon), right-click → "Oyun Odası" created (the observer client saw it), joined
  from the menu; an unregistered user trying a permanent and a temporary channel got "You don't have permission for
  this action." (Mumble's default ACL).

## Addendum: the desktop Mumble user right-click menu (2026-10-13)

The 18 items in the screenshot the user shared (right-click on another user as SuperUser) were implemented in the
order and with the permission logic of the desktop client (`MainWindow::qmUser_aboutToShow`,
`updateMenuPermissions`, v1.5.939).
- Permissions: root permissions from `ServerSync.permissions` (`ready.permissions`), channel permissions from
  `PermissionQuery` (on demand, reset by `flush`). The server sends no permission info to SuperUser; SuperUser =
  `All & ~(Speak|Whisper)` (`ChanACL::effectivePermissions`). `userId`, `hash`, `hasComment`, `hasTexture` were
  added to what the browser receives.
- Gateway commands: `moveUser`, `setUser` (mute/deaf/prioritySpeaker), `kick`, `ban`, `register`, `resetComment`,
  `resetTexture`, `requestComment` (RequestBlob), `userStats` (UserStats → `userStats` message), private `text`
  (`session`), `queryPermissions`.
- **Important finding: ban.** The server's `msgUserRemove(ban)` also bans the user's IP; since all web users share the
  gateway IP this locks everyone out. The web "Ban" therefore fetches the ban list, adds an entry with the address
  `100::/128` (RFC 6666, matches no client) + the certificate hash, writes it back and kicks the user. A warning about
  bans made from the desktop was added to the README.
- Local preferences (by certificate hash, else by name): nickname (nickname in the tree, real name in the tooltip),
  ignore messages (do not reach the chat), friend (heart icon; "update" if the name changes). A priority-speaker
  icon was added.
- Windows: kick/ban reason, nickname, private message (Ctrl+Enter), comment (DOMPurify), info (client, OS, times,
  address, ping, certificate, packets).
- When logging in as SuperUser the chat shows a "SuperUser cannot speak" notice.
- Verification: Go integration test `TestAdminActions` (permission query as a normal user, mute/deafen/priority,
  move and back, private message reaching only the target, info, register, kick; after the ban the banned
  certificate cannot get back in and **another web user can**; the server log shows `Host: "[100:]"`). Browser
  (Chromium, two separate contexts): the menu content is in the same order as the desktop; move, mute, deafen,
  priority (icon), nickname, private message on both sides, ignored messages not shown, info window, view comment
  (`<script>` stripped) and reset (then disabled), friend (heart), register (registered), a normal user's menu has
  the admin items missing/disabled, kick → "You were kicked from the server: test over" on the target.
- Known limit: editing your own comment and showing avatars are missing (an avatar can only be removed).

## Addendum: the desktop Mumble channel right-click menu (2026-10-13)

The channel menu in the user's screenshot, in the order and with the rules of the desktop client
(`qmChannel_aboutToShow` + `updateMenuPermissions`, v1.5.939; both rule sets applied together):
Join channel, Listen to channel | Add…, Edit…, Remove… | Link, Unlink, Unlink all | Copy URL, Send message… |
Hide when filtering, Pin when filtering.
- Gateway: `editChannel` (name/description/position/user limit), `removeChannel`, `linkChannels` (link/unlink always
  via our own channel, like the desktop), `listen` (`listening_channel_add/remove`), `requestDescription`
  (RequestBlob), `text` + `channelId` + `tree` (`tree_id`). `hasDescription`, `maxUsers` added to the channel
  message; `listening` to the user.
- **Fix:** Mumble channel links are bidirectional (`Channel::link`) but the server reports a change through one
  channel only; the state now updates the other channel too and emits events for both (unit + integration test).
  When a channel is removed it also drops from the other channels' links and listener lists.
- Copy URL: instead of the desktop's `mumble://` address (Mumble's external address is not known to the gateway) the
  web address `…/#kanal=/Path/Sub`; someone logging in with that link joins the channel at the first `ready`.
- Filter: a funnel button in the channel list header; when on, channels without users (or listeners) are hidden,
  "pin" always shows, "hide" (unless it is our channel) always hides. Per-channel settings are local.
- Tree: listeners under the channel with an ear icon, a chain icon on linked channels. Sent channel/private
  messages get a "You → #Channel" / "You → Name" label (only private messages are marked "private").
- Verification: Go `TestChannelActions` (create, edit, description, link/unlink and the order of the two sides sent to
  the browser, listening and the `listening` field, tree message, delete), `TestLinksAreSymmetric`,
  `TestListeningChannels`. Browser (SuperUser + a normal user in a separate context + a third person arriving by
  link): menu content, add, edit (values come back when the form reopens), link/unlink/unlink-all icons, listener row
  and tick, tree message to a user in a sub-channel, copy URL → a new session lands straight in that channel, filter +
  pin/hide, remove; admin items disabled in a normal user's menu.
- Known limit: channel password (via ACL) and viewing channel descriptions are missing.

## Addendum: password-protected channels (2026-10-13)

In Mumble a channel password is an ACL (desktop `ACLEditor::updatePasswordACL`): `@all` is denied
Enter|Speak|Whisper|TextMessage|LinkChannel|Traverse and the `#<password>` group is allowed; the password is sent as
an "access token". Desktop 1.5 does not ask for the password on entering (the Access tokens window); the requested
flow was implemented on the web.
- Entering: when joining a locked channel (the server's `is_enter_restricted` / `can_enter` fields) or on an "Enter"
  denial, a password window appears. The password is added to the session's **persistent** access tokens (mid-session
  via `Authenticate.tokens`; the server re-sends `can_enter` for all channels), then the join request is sent. The
  temporary token (`UserState.temporary_access_tokens`) was not used because it only counts at the entry check: the
  password ACL also restricts Speak/TextMessage, so the user could not speak inside. Wrong password → the token is
  withdrawn and the window reopens with an error. "Remember in this browser" → localStorage, sent at login in
  `hello.tokens`; removable from Settings.
- Admin: "Channel password" in the Edit window (the ACL is read and shown; on change it is written back with the same
  rules as the desktop: inherited entries and unchanged inherited groups are skipped, an existing password ACL +
  its matching `@all` denial are replaced).
- Tree: a locked channel has a red lock, a channel whose password you hold has an open lock.
- **Finding: server message rate limit.** murmur applies `messagelimit` (1/s) / `messageburst` (5) per user (own
  UserState, channel creation, TextMessage, ACL) and silently drops the excess. An ACL write is 2 messages (query +
  write). Rapid consecutive operations in tests were dropped because of this; a note was added to the README.
- Verification: Go `TestChannelPassword` (set/read the password, locked and denied for a normal user, a wrong token
  denied, the right token → `canEnter` true and entry, `hello.tokens` valid from login, restriction gone once the
  password is removed). Browser: SuperUser set a password via Edit; a normal user saw a red lock, a password window on
  double-click, wrong password → error line, right password + remember → in the channel and able to chat, entered
  without a password after a reload, locked again after the password was removed in Settings; the admin sees the
  current password in Edit.

## Addendum: web users on the same IP: nickname takeover (2026-10-13)

Source (murmur `msgAuthenticate`, "Allow reuse of name from same IP"): if an unregistered user arrives from the
**same IP** with a name already in use, the server drops the old session instead of answering UsernameInUse. Since
all web users share the gateway IP, any web user could kick another web user by typing their name.
- The gateway keeps the names (case insensitive) and certificate hashes of open sessions and rejects a login with
  the same name from a **different certificate** with `reject UsernameInUse` without connecting to Mumble. The same
  certificate (reconnect, a second tab) is accepted as before and the server closes the old session.
- The hash is computed the way Mumble does (SHA-1 of the DER certificate, hex); compared with the hash the server
  sees in the integration test.
- Verification: `TestNameRegistry` (claim, case, a second session with the same certificate, release) and
  `TestNameTakeover` (real server: hash match, the same name with another certificate rejected and the owner stays
  connected, reconnecting with the same certificate replaced the old session).
- Desktop users are not affected (a different IP → the server already refuses). Other places where the IP matters on
  the server: ban (open issue), autoban (solved), the UserStats address (cosmetic).

## Phase 9 decision (2026-10-13)
A WASM Opus fallback will **not** be built (user decision). Rationale: per the WebKit source Safari 26+ (iOS 26 /
macOS) supports raw mono/stereo Opus through WebCodecs `AudioEncoder`/`AudioDecoder` (`AudioEncoderCocoa.cpp`,
`AudioDecoderCocoa.cpp`); a fallback would only be for older Safari, and Opus encoder libraries for the browser are
dated. Priority goes to Android and desktop web features. Safari 26+ has not been tested on a real device.

## Documentation: English as the primary language (2026-10-13)

- `README.md`, `docs/DEVELOPMENT.md`, `docs/PROGRESS.md`, `docs/TODO.md` and the new `docs/TESTING.md` are now
  English. Turkish versions live in `README.tr.md` and `docs/tr/`; every page links to its counterpart.
- The README was restructured for a public audience (highlights, feature table, browser support, quick start,
  deployment, configuration, building, testing, troubleshooting). Repository-internal material moved into the new
  `CLAUDE.md`.
- Deploy files: the Turkish welcome-text default in `deploy/docker-compose.yml` and `.env.example` and the error
  message of the `certs` service in `deploy/local/compose.yaml` are now English.

## Addendum: English user interface and language switch (2026-10-13)

- `web/src/i18n/en.ts` mirrors `tr.ts` (typed as `Translations`, so a missing or extra key fails `npm run check`).
  `i18n/index.svelte.ts` holds the reactive language (`language`), the `t` proxy that components import, and
  `locale()` for `Intl` / collation. Components switch language instantly without a reload or reconnect.
- Language choice: the saved choice (`mumweb.lang` in localStorage), else the first English or Turkish entry of
  `navigator.languages`, else Turkish (`FALLBACK`; chosen by the owner for non-English/non-Turkish browsers).
- UI: an **EN / TR** switch (`LangSwitch.svelte`) at the top right of the login screen and at the right end of the
  status bar, so it is visible before and after login. `<html lang>` and the meta description follow the choice.
- Moved into the dictionaries: key names (`lib/keys.ts`), aria-labels, the chat time format and the channel/user
  sort collation. `index.html` and `manifest.webmanifest` are English (static, cannot follow the choice).
- Verification: `npm run check` 0 errors, `npm test` (19) and `npm run build` pass. Not yet checked in a browser by
  hand: switching while connected, and the fallback with a non-English/Turkish browser language.

## Addendum: web access password (2026-10-13)

- `MUMWEB_ACCESS_PASSWORD` (empty = off). The browser sends it as `hello.accessPassword`; it is separate from
  `hello.password`, which stays the Mumble server password. The check runs first in the session, before username
  validation, certificate handling and any Mumble connection. Reject reasons: `AccessPasswordRequired`,
  `AccessPasswordWrong`, `AccessRateLimited` (not Mumble types; the UI maps them to messages).
- Constant-time comparison (SHA-256 of both sides, `subtle.ConstantTimeCompare`). A wrong guess is answered after
  1 s; 5 wrong guesses from one IP within 10 minutes lock that IP out, even for the right password (`failTracker`).
  The existing per-IP connection rate limit still applies. Passwords are never logged.
- `GET /config` returns `{"accessPassword": bool}` (`Cache-Control: no-store`; excluded from the service worker).
  The login screen fetches it and shows the "Access password" field only when needed, with "Remember in this
  browser" (localStorage). A reject with an `Access…` reason also reveals the field, and a wrong password forgets the
  saved one.
- Limits: a door code, not per-user authentication; desktop Mumble clients bypass it (port 64738); it is as private
  as the HTTPS connection that carries it.
- Verification: Go tests `TestAccessPassword` (required, wrong, the Mumble server password is not accepted, right
  password passes, lockout after 5 wrong guesses also for the right password), `TestFailTracker`,
  `TestPasswordMatches`, `TestNoAccessPassword`; the built gateway answers `/config` with `true` when the variable
  is set. `npm run check` 0 errors, `npm test` and `npm run build` pass. **Not tried by hand in a browser.**

## Addendum: first real-device test on Android (2026-10-13)

Reported by the owner: an Android phone (Samsung Internet) connected to the local environment and talked with a
desktop client using real microphones.
- Works: voice and chat both ways between the PC and the phone; the responsive layout is good; the session keeps
  running when the screen locks.
- Firefox for Android 157.0.1: the page reports that the codec is not supported. Cause not investigated yet.
- Sent to the background: the user leaves the room after 15–20 s. Cause not investigated yet.
- No "Add to home screen" option appeared; most likely because the phone does not trust the mkcert CA (the page is
  not a trusted secure origin for installation). To be retested over a real certificate.
- Consequences: the "real microphone, both ways" manual test is closed; the three problems above are in TODO (B, C, D).

## Addendum: published Docker image (2026-10-13)

- `aoztrk82/mumweb` on Docker Hub (public), tags `latest` and `0.1.0`, platforms `linux/amd64` and `linux/arm64`
  (index digest `sha256:03ce904f…`). The Dockerfile builds the web and Go stages on the build platform
  (`--platform=$BUILDPLATFORM`) and cross-compiles Go with `GOOS`/`GOARCH` from `TARGETOS`/`TARGETARCH`, so no emulation
  is needed for the heavy steps. `make publish` wraps the `docker buildx build --push` command.
- `deploy/docker-compose.yml` now pulls `aoztrk82/mumweb:${MUMWEB_VERSION:-latest}` instead of building;
  `deploy/docker-compose.build.yml` is the override for building from source. The README quick start no longer needs
  `git clone`: it lists the three files (compose, Caddyfile, `.env`); the compose example is a copy of the repo file.
  Updating is `docker compose pull && docker compose up -d`.
- Building, testing, repository layout and the release procedure moved from the README to `docs/BUILDING.md`.
- Verification: pulled the pushed `0.1.0` image on a clean tag, started it with `MUMWEB_ACCESS_PASSWORD` set:
  `/config` → `{"accessPassword":true}`, `/healthz` → `ok`, runs as uid 100 (`mumweb`). The manifest lists amd64 and
  arm64. **The arm64 image was not run** (no emulation on the build machine); it built cleanly. The updated
  production compose was validated with `docker compose config` only; a full install on a clean server with the new
  README steps is still to be done.

- Docker Hub page: short description and a README (`docs/DOCKERHUB.md`: what the image is, tags, a minimal compose
  snippet, the variable table, links back to GitHub for the full setup) were set through the Docker Hub API with
  `scripts/dockerhub-description.py` (`make hub-readme`), reusing Docker Desktop's stored login. Checked on the public
  repository page API.

## Addendum: web admin panel (2026-10-14)

- **What:** `/admin/` is a second Vite page (`web/admin/index.html`, `web/src/admin/`) served by the gateway
  (`/admin` redirects to `/admin/`). It manages online users, the channel tree and channel passwords, a full ACL and
  group editor, the ban list and the registered users. `/config` now also reports `{"admin": bool}`.
- **Decision: the panel is a normal bridge session with `hello.role = "admin"`.** It needs `MUMWEB_ADMIN_PASSWORD`
  (empty = panel off, role refused with `AdminDisabled`). The admin password replaces the web access password for
  this page and has its own `failTracker`: behind a proxy without `MUMWEB_TRUST_PROXY` all users share one IP, so a
  shared tracker would let admin brute-forcing lock everybody out of the main UI. Why a password in front of the
  Mumble login: a stranger must not even be able to try SuperUser's password through the gateway.
- **Decision: SuperUser is allowed only on admin sessions** (after the admin password). `MUMWEB_ALLOW_SUPERUSER` stays
  as it is for the main client, so production keeps the main login closed. This also answers "SuperUser cannot speak":
  the panel has no audio at all, so nobody expects it to.
- **Decision: voice and admin side by side in one browser.** Two tabs of one browser shared one IndexedDB certificate
  and Mumble drops the old session when the same registered account connects again (and the gateway refuses the same
  name with another certificate). The panel therefore uses its own certificate key (`cert-admin`) and must log in with a
  different account (SuperUser) than the voice tab. Admin sessions are deafened right after login, the gateway drops
  audio events for them and treats a binary frame from them as a protocol violation.
- **Gateway-side checks on top of the server's:** admin commands (`adminBanList`, `adminSetBanList`, `adminUserList`,
  `adminUpdateUsers`, `adminACL`, `adminSetACL`) on a non-admin session end it with a protocol error. A ban list write
  may only keep entries from the list the server last sent or add certificate-only ones (`100::/128` with a hash): a new
  IP ban would hit every web user, but deleting an IP ban made from the desktop is allowed. SuperUser (id 0) cannot be
  renamed or deleted; permission masks, group names and list sizes are bounded; the admin JSON read limit is 512 KiB
  (ban lists, ACLs) instead of 16 KiB.
- **Rate limit:** the Mumble server silently drops messages beyond ~1/s (burst 5), queries included. An integration run
  that fired ten commands in a second lost the last query. Admin commands are now paced in the gateway (0.8/s, burst 4,
  a command waits up to 6 s, else the panel gets `adminBusy`) and the UI re-reads a list after each write instead of
  trusting its draft.
- **Stale lists:** the Bans, Registered users and ACL views re-read their data every time they open (and after saving)
  instead of caching it; otherwise a user banned or registered from Online would be missing, and saving a stale ban list
  would delete the newer entry. The remaining race (a change made elsewhere while the Bans tab stays open) is in TODO.
  The admin lockout has its own reject reason (`AdminRateLimited`).
- **Audit log:** every state-changing admin command is logged with action, channel/target ids and counts. Never
  passwords, chat or ACL group names (a `#name` group is a channel password).
- **Protocol additions (Go `internal/mumble/admin.go`):** `UserList` was not wired at all; `BanList` and `ACL` replies
  were swallowed unless a kick-ban or password request was pending. They now emit `BanListEvent`, `UserListEvent` and
  `ACLEvent`; ACL write-back reuses the desktop client's filter (own non-inherited entries, groups that differ from plain
  inheritance). A password query and an ACL report on the same channel no longer collide.
- **Service worker bug avoided:** `sw.js` stored every navigation response under the `/` key, so opening `/admin/`
  would have replaced the cached app shell with the panel (and an offline `/admin` would have shown the voice app).
  It now caches only `/`, never touches `/admin*`, and the cache name is `mumweb-static-v2`.
- **Verification:**
  - Go unit tests: ban list, user list and ACL round trip against a fake TLS Mumble server (unchanged write-back
    reproduces only own entries), password/lockout isolation, ban/ACL/user-update validation, pacing.
  - Go integration test `TestAdminPanelEndToEnd` against a real **Mumble 1.5.915** server as SuperUser: normal session
    cannot use admin commands; user list; ban add/read/remove; scratch channel ACL write and read-back (entry with
    grant/deny and `applySubs=false`, group with a member); a new IP ban ends the session.
  - Headless Chrome (puppeteer-core, scratch scripts, not committed) against the rebuilt local deployment
    (`deploy/local`, `https://mumweb.test`): wrong admin password refused; SuperUser login; all four tabs render; channel
    creation through a prompt; the ACL editor opens on Root with its four details sections (admin, auth, all rules and
    the admin group); a certificate ban was added, saved, re-read and removed; no console errors. A voice session
    (`ses-e2e`) and the panel stayed connected together for 6 s in one browser and the panel listed the voice user.
    The test channel and ban were removed again afterwards.
  - Final pass after the review fixes: the whole `go test ./internal/bridge` suite (also the older kick/ban, channel
    password and name-takeover integration tests) against a fresh Mumble 1.5.915 container, then the local deployment
    rebuilt and the browser scripts repeated: from **Online** the voice user was registered (visible under Registered
    users, deleted again) and certificate-banned (visible under Bans, their voice session ended, entry removed and saved).
    The Bans / Registered tabs had been opened before the action, so a stale cache would have shown.
  - Not done: a phone/narrow-screen check, the desktop Mumble client's ACL editor showing a panel-written ACL, long-run
    use. The Docker Hub page was **not** updated (the published image has no panel yet).
- Local deployment: `MUMWEB_ADMIN_PASSWORD` defaults to `admin` in `deploy/local/compose.yaml` and the dev compose file.

## Addendum: release 0.2.0 (2026-10-14)

- `aoztrk82/mumweb:0.2.0` and `:latest` pushed with `docker buildx build --builder mumweb-builder --platform
  linux/amd64,linux/arm64 --push` (the same command `make publish` wraps; GNU Make is not installed on the Windows dev
  machine). Index digest `sha256:c25f8ce3…`. Contents: the admin panel. `release` in `main.go` is now `mumweb 0.2`.
- Verification: the manifest lists amd64 and arm64; the pulled image started with `MUMWEB_ADMIN_PASSWORD` answers
  `/config` → `{"accessPassword":false,"admin":true}`, `/healthz` → `ok`, `/admin` → 301 to `/admin/` (200), runs as uid 100.
  The arm64 image was again not run. Docker Hub page refreshed with `scripts/dockerhub-description.py` (lists
  `MUMWEB_ADMIN_PASSWORD`).

## Addendum: security review, first pass (2026-10-14)

Full write-up in [SECURITY.md](SECURITY.md); this entry records the decisions and the verification.

- **Scans:** `govulncheck` (needs the tool built with the project's Go version, `GOTOOLCHAIN=go1.27.2`, else it cannot read
  Go 1.27 code), `npm audit`, Trivy on the published `aoztrk82/mumweb:0.2.0` (needed `--db-repository
  ghcr.io/aquasecurity/trivy-db:2`, the default mirror returned 404): all clean. `gosec`: 3 accepted findings;
  `staticcheck`: one deliberate repeated call in a test.
- **Changes:** CSP and other hardening headers from the gateway (`securityHeaders` in `cmd/mumweb`), HSTS from Caddy;
  per-IP concurrent session limit (`MUMWEB_MAX_SESSIONS_PER_IP`, 10) because the per-IP rate alone still let one address fill
  all slots; a control-message token bucket (50/s, burst 200 so the login-time permission queries pass; 500 dropped in a
  row ends the session); HTTP `IdleTimeout` and `MaxHeaderBytes`; compose hardening (gateway read-only, no capabilities,
  limits, `internal` network without egress; Caddy keeps `NET_BIND_SERVICE`; Mumble only `no-new-privileges` and a
  process limit).
- **Decision: Mumble keeps its capabilities.** `cap_drop: ALL` made its entrypoint exit ("Failed to set initial
  capabilities", then a permission error on `/data`). Left out on purpose; recorded in SECURITY.md.
- **Decision: no non-extractable WebCrypto key.** The gateway builds the TLS client certificate and therefore needs the PEM
  on every connect; the strict CSP and the XSS tests are the mitigation, the exposure is an accepted risk.
- **Decision: HSTS in Caddy, not in the gateway** (the gateway may be served over HTTP in development). The Caddyfile in the
  docs restricts `/admin` to one address; it was tested on the local Caddy (404 for other addresses, `/` 200).
- **Verification:**
  - Go: five fuzz targets (`FuzzParseAudioFrame`, `FuzzClientMessage`, `FuzzReadFrame`, `FuzzDecodeAudio`, `FuzzStateMessages`),
    25 s each, no crash; new unit tests for the headers, per-IP limit (also over HTTP: the third socket from one IP gets
    429) and, against a real Mumble 1.5.915, a control flood that ends the session while a 150-message login burst is
    accepted. The whole integration suite passed again.
  - Web: 25 XSS unit tests for the sanitizer (jsdom, a new dev dependency) and `textToHtml`.
  - Local deployment rebuilt with the hardened compose: headers seen through Caddy (CSP, HSTS, X-Frame-Options, ...);
    the gateway container is read-only (`touch /x` fails), has no route out (`wget http://1.1.1.1` → network
    unreachable), runs with `CapDrop ALL`, 256 MiB and 200 processes; headless Chrome ran the voice client (login, chat
    with an `<img onerror>` payload shown as text) and all admin tabs with **no console or CSP errors**.
  - On this Windows machine the local Mumble container cannot publish UDP 64738 (port reservation), so it was started
    through a throw-away override that drops the Mumble port mapping; that file is not part of the repository.
- Not done yet: see SECURITY.md section 7 (CI scans, Semgrep/ESLint, long fuzzing, digest pinning, distroless).

## Addendum: release 0.3.0 (2026-10-14)

- Ships the security first pass: security headers and CSP, per-IP session limit (`MUMWEB_MAX_SESSIONS_PER_IP`), control-message
  rate limit, HTTP idle/header limits, hardened compose files, HSTS in the Caddyfiles. `release` in `main.go` is `mumweb 0.3`.
  Version chosen as a minor bump because of the new setting. Pushed as `:0.3.0` and `:latest` (amd64+arm64, index digest `sha256:d6657463…`); Trivy on it: 0 HIGH/CRITICAL; the container answers with the new headers. GitHub private vulnerability reporting was enabled by the owner.

## Addendum: reserved web slots (2026-10-14)

- New `MUMWEB_RESERVED_SLOTS` + `MUMWEB_RESERVED_PASSWORD`: of `MUMWEB_MAX_SESSIONS` web slots, N can only be taken by
  browsers that send the reserved password in hello (optional login field, shown when `/config` says `reservedSlots`) and
  by admin sessions. Decisions: the total stays the hard limit applied at socket accept; the slot split is applied after
  the login checks (`acquireSlot`), so privileged users fill general slots first and keep the reserved ones for when the
  general area is full; a wrong reserved password is rejected (own failure tracker, same delay and lockout as the access
  password) instead of silently falling back; off unless both values are set and a general slot remains. New reject
  reasons `WebFull`, `ReservedPasswordWrong`, `ReservedRateLimited`. The web limit itself already existed
  (`MUMWEB_MAX_SESSIONS`, independent of the Mumble server's user limit). Unit tests: `slots_test.go`.

## Addendum: release 0.3.1 (2026-10-14)

- Adds the reserved web slots. `aoztrk82/mumweb:0.3.1` and `:latest` pushed (amd64+arm64, index digest `sha256:0e627df0…`);
  Trivy: 0 HIGH/CRITICAL; the image answers `/config` with `reservedSlots:true` when configured; Docker Hub page refreshed.
  The server test deployment (`deploy/test/`) pulls this image.

## Addendum: live security and load test on mumweb.aozturk.site (2026-10-14)

- Ran a round of black-box tests from outside against the running test deployment (release 0.3.1): response
  headers/TLS, exposed ports, WebSocket Origin check, access-password lockout (and its 10 minute expiry), reserved
  web slots (right/wrong password), admin password (separate lockout from the access password), the per-IP session
  limit under 120 concurrent attempts, oversized/malformed first frames, a control-message flood (ends the session
  after 500 drops, as designed), an audio-frame flood (dropped only, session stays open, by design), idle/no-hello
  timeouts (no slowloris leak), a chat flood, a real-browser XSS attempt in chat and in the username field (Mumble's
  own server rejects `<`/`>` in usernames before the gateway or browser see them), and 60 concurrent full logins
  (completed in under 4 s, `/healthz` unaffected). Everything held; no code change was needed. Full detail and the
  reasoning for each: [SECURITY.md](SECURITY.md).

## Addendum: dropped image digest pinning, rewrote the roadmap, reworked the deployment docs (2026-10-14)

- **Digest pinning reverted.** Pinning `MUMWEB_IMAGE`/`MUMBLE_IMAGE`/`CADDY_IMAGE` by `sha256` digest in
  `deploy/docker-compose.yml` (added earlier the same day) was reverted: the owner judged it over-engineered for this
  project's size, and it meant `docker compose pull && docker compose up -d` — the standard update flow every
  self-hoster expects — silently did nothing until someone manually looked up and edited a new digest. Production
  images go back to floating tags (`mumblevoip/mumble-server:latest`, `aoztrk82/mumweb:${MUMWEB_VERSION:-latest}`,
  `caddy:2`); pinning a specific `MUMWEB_VERSION` tag in `.env` remains the documented way to hold a release.
  `docs/SECURITY.md` §5/§7 describe pin-by-tag again as the recommendation, digest pinning as a possible but undone
  hardening step.
- **`docs/TODO.md` / `docs/tr/TODO.md` rewritten to a feature-only roadmap.** The security review checklist (former
  section 4, now fully covered by [SECURITY.md](SECURITY.md)) and the manual-test/CI/technical-debt sections were
  dropped from TODO; completed items live in this log instead of as a wall of ticked boxes. What remains is a flat
  "Planned features" list plus a short "Known limitations" list, so TODO answers one question only: what feature
  work is left.
- **README deployment section restructured** around how real self-hosters actually run this: most already have a
  compose file with a reverse proxy (and often their own Mumble server), so the README now leads with **Option A**
  (add the `mumweb` service to your existing compose file, with Caddy/nginx/Traefik reverse-proxy snippets) and
  only offers **Option B** (the full three-service compose from this repo) for a from-scratch server. The old
  "quick start" / "deployment" split is gone; it is one "## Deployment" section now, with a services table
  ("how the pieces fit") up top. Cross-references to the old `#quick-start-production` anchor
  (`docs/BUILDING.md`, `docs/DEVELOPMENT.md`, `docs/DOCKERHUB.md`, and the Turkish equivalent in
  `docs/tr/BUILDING.md`) were updated to `#deployment`. Driven by an upcoming Reddit announcement: the owner wants
  the README to read like a general self-hosting guide, not a "copy our compose file" tutorial.

## Addendum: README screenshot and demo GIF (2026-10-14)

- Removed two unused leftover dev screenshots (`docs/superuser_right_click_channel_menu.png`,
  `docs/superuser_right_click_user_menu.png`) that nothing referenced.
- Added `docs/images/screenshot.png` and `docs/images/demo.gif`, embedded near the top of `README.md` and
  `README.tr.md`, right after the intro paragraph. Captured against the local HTTPS dev stack (`deploy/local`,
  `https://mumweb.test`) with two scripted browser sessions ("Alice", "Sam") joining a channel and chatting, plus a
  right-click user context menu to show that feature off; the leftover `test`/`wwww`/`test33` channels from earlier
  admin-panel testing were renamed to `General`/`Gaming` via the admin panel first so the channel tree looks
  presentable. Captured with puppeteer-core + headless Chrome, assembled into a GIF with `ffmpeg` (palette-based,
  ~900px wide, ~145 KB). Scripts are scratch (not added to the repo).
  - Found along the way: running two tabs in the **same** headless Chrome instance with
    `--use-fake-device-for-media-stream` reliably deadlocks Puppeteer's `Runtime.callFunctionOn` (times out even at
    a 180 s protocol timeout) once both tabs are producing fake audio; a single tab, or multiple tabs without the
    fake device, works fine. Worked around by giving each simulated user its own full browser process instead of a
    second tab — not a mumweb bug, just a headless-Chrome/fake-audio quirk worth remembering for future UI capture
    scripts.

## Addendum: operator-set server name and description (2026-10-14)

- Added `MUMWEB_SERVER_NAME` and `MUMWEB_SERVER_DESCRIPTION` (both optional, empty by default, plain text not HTML),
  returned by `/config` alongside the existing booleans. The login screen shows the server name as its heading
  (falling back to "mumweb") with a new "Mumble Web Client" tagline underneath, and the description where the
  "Join the voice chat" subtitle used to be unconditionally; the status bar shows both next to "Connected" once
  joined, so the previously near-empty header has real content without any extra network request (`/config` is
  already fetched on login). `document.title` is also set to `"<name> — mumweb"` once the name is known.
  - Deliberately independent of Mumble's own welcome text (`session.welcome`, shown as the first chat line): that
    comes from the Mumble server and needs an actual connection, while this is operator-set branding available
    before login. No new backend connection was added to fetch it earlier; that would have meant a persistent
    "probe" connection to Mumble outside the one-connection-per-browser-session model, which was deliberately
    avoided.
  - Verified against the local dev stack with both values set in `deploy/local/.env`: `/config` returns them,
    the login heading/tagline/subtitle and the tab title render correctly, and the status bar shows
    "Connected · Friday Night Voice  Our little Mumble server for game nights and chats." after joining (screenshot
    checked, not committed). `go vet`, `go test ./...`, `npm run check` and `npm test` all pass.
  - Added to the production, local and test compose files (empty defaults, no behavior change for existing
    deployments) and to `deploy/.env.example`, the README env var tables (English and Turkish), and
    `docs/DOCKERHUB.md`; `make hub-readme` still needs a maintainer to run it to push the Docker Hub page text.

## Addendum: five new UI languages (2026-10-14)

- Added Arabic, German, Spanish, Italian and Russian dictionaries (`web/src/i18n/{ar,de,es,it,ru}.ts`), alongside
  the existing English and Turkish ones, bringing the UI to seven languages. `tr.ts` stays the canonical shape
  (`Translations` type); each new file was written by mirroring its keys and translating only the values, keeping
  Mumble/protocol proper nouns (SuperUser, ACL, Opus, PEM, SHA-1, TCP/UDP) in Latin script and informal address
  (du/tú/ты) for German/Spanish/Russian, matching the existing Turkish tone. `lang` was extended to seven keys in
  every dictionary (native autonyms, translated `label`).
  - Arabic is RTL: `apply()` in `index.svelte.ts` now sets `document.documentElement.dir` from a `RTL` set
    (`{'ar'}`), applied at bootstrap too, not just on manual switch. No RTL-specific CSS was needed anywhere —
    flexbox plus the native `dir` attribute mirrored the whole login card correctly on its own.
  - `LangSwitch.svelte` kept its button-row layout (not a dropdown) with `flex-wrap` added so seven two-letter
    buttons wrap instead of overflowing.
  - Verified: `npm run check` (0 errors, all seven dictionaries type-check against `Translations`), `npm test` (50
    passed), `npm run build`, and a visual check against the local dev stack — all seven language buttons render,
    clicking AR flips `dir`/`lang` and mirrors the login card, clicking back to DE restores `dir="ltr"` with correct
    German text.

## Addendum: release 0.4.0 (2026-10-14)

- Bumped `release` in `gateway/cmd/mumweb/main.go` and `VERSION` in the `Makefile` from 0.3.1 to 0.4.0: covers the
  digest-pinning revert, the README/TODO restructure, the screenshot/demo GIF, operator-set server name/description,
  and the five new UI languages — all feature/doc work above, no breaking changes. `aoztrk82/mumweb:0.4.0` and
  `:latest` pushed with `make publish VERSION=0.4.0` (amd64+arm64).

## Addendum: show/copy on password fields (2026-10-14)

- User feedback: typing a password blind into a field is error-prone, and if it's wrong there's no way to tell
  where. Added a reusable `PasswordField.svelte` (an input plus an eye toggle and a copy button, absolutely
  positioned over the right side of the field) and swapped it in everywhere a `type="password"` input existed: the
  login screen's access/reserved/server-password fields, the admin login's admin/Mumble-password fields, and the
  generic password prompt dialog used for joining a password-protected channel. The channel-password *editor* in
  the edit-channel dialog and the admin panel's channel-password editor were left as plain visible `type="text"`
  fields, unchanged — those are already shown in the clear by design (an operator/channel-owner setting a password,
  not someone typing one in blind).
  - New `t.password` i18n section (`show`/`hide`/`copy`/`copied`) added to all seven dictionaries.
  - Copy uses `navigator.clipboard.writeText`; failures (no Clipboard API, permission denied) are swallowed
    silently since there's nothing actionable to show the user without a toast system, and the button's own
    "Copied" state simply never appears. Verified the component's eye toggle and the clipboard write call with a
    puppeteer script: toggling switches the input's `type` between `password`/`text` and the typed value renders;
    clipboard write was attempted with the correct value but denied by headless Chrome's permission model
    (`overridePermissions` doesn't make `navigator.clipboard.writeText` succeed in headless) — a known
    headless-automation limitation, not a product issue, consistent with the other Puppeteer/headless gotchas
    logged above.
  - Verified: `npm run check` (0 errors), `npm test` (50 passed), `npm run build`, and visual checks against the
    local dev stack in English and Turkish (screenshots).

## Addendum: invite links (2026-10-14)

- Added `MUMWEB_INVITE_SECRET`: when set, the admin panel's new **Invites** tab mints links
  (`/?invite=<token>`) that skip the web access password until they expire. A token is **stateless**: 8 bytes of
  Unix expiry plus an HMAC-SHA256 of those bytes keyed by the secret, base64url-encoded
  (`gateway/internal/bridge/invite.go`). The gateway keeps no list of issued tokens, so one cannot be revoked
  individually before it expires; rotating `MUMWEB_INVITE_SECRET` invalidates all of them at once, which is the
  only revoke mechanism. Unlike a password, a token is unguessable (a 256-bit MAC), so `checkInvite` gets no
  per-IP lockout tracker — a wrong or expired token is just refused (`InviteInvalid`/`InviteExpired`).
  - `session.run()`: a non-admin hello with a non-empty `inviteToken` (and the feature on) is checked instead of
    the access password, not in addition to it — the invite fully substitutes for that one gate. It does not touch
    the separately-configured reserved-slot password or grant a reserved slot; those stay independent.
  - Admin side: `adminCreateInvite` (`{inviteTtlSeconds}`, capped at `maxInviteTTL` = 30 days) returns
    `inviteCreated` (`{token, expiresAt}`); audited like other admin writes (TTL only, never the token). The admin
    UI (`web/src/admin/InvitesTab.svelte`) builds the full URL client-side (`location.origin + '?invite=' + token`)
    and keeps a session-local list (not persisted, not sent back to the server) with copy/remove per entry — "remove"
    only drops it from this list, it does not revoke the token.
  - Web login (`Login.svelte`): reads `?invite=` from the URL once on mount, stores it in `session.inviteToken`,
    and immediately strips it from the address bar with `history.replaceState` so it does not linger in browser
    history or a screenshot. While an invite token is held, the access-password field is replaced with a short
    notice ("You're joining with an invite link…") instead of being shown empty and required. A reject whose
    reason starts with `Invite` clears `session.inviteToken` so a retry falls back to the normal login form.
  - `deploy/local/compose.yaml` now sets `MUMWEB_INVITE_SECRET` to a fixed dev value by default (like the admin
    and access passwords already did) so the feature is on out of the box in local dev.
  - Verified end-to-end against the local HTTPS dev stack with puppeteer: logged into `/admin/` as SuperUser,
    opened the Invites tab (only tab list that appears when `/config` reports `invite: true`), created a 1-day
    link, copied it (button briefly shows "Copied"), opened it in a fresh page — the access-password field was
    replaced by the notice, the invite param was gone from the address bar after load, and connecting as a new
    nickname worked without any password. A garbage token on `/?invite=...` was correctly rejected with "This
    invite link is invalid." `go test ./...` (new `invite_test.go`: sign/verify round trip, tampering, wrong
    secret, expired/invalid/disabled-feature rejection, and that a valid invite bypasses a configured access
    password while its absence still requires one), `go vet`, `gofmt`, `npm run check` (0 errors across all seven
    i18n dictionaries), `npm test` (50 passed) and `npm run build` all pass.

## Open issues
- When a web user is banned from the desktop client the ban entry also contains the gateway IP and blocks new web
  logins (existing sessions stay). **Decision (2026-10-13): no mitigation for now**: the target use is small
  groups/teams and bans are rare. The web "Ban" only bans the certificate; after a desktop ban an admin deletes the
  IP entry from the ban list (described in the README). A "guard account" automatic fix may be considered later.
- Mumble autoban in production (Phase 7): at least `autobanSuccessfulConnections=false`. Failed logins (wrong
  password) are still counted against the gateway IP; 10 wrong passwords in 120 s cut off all web users for 5 minutes.
- The plan's 5.3 `frame_number` "+1 / 20 ms" should read "+2 / 20 ms" (10 ms unit) per the official client (see the
  Phase 4 note).
