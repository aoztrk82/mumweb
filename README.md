# mumweb

**English** | [Türkçe](README.tr.md)

A self-hosted web client for [Mumble](https://www.mumble.info/) voice chat. Your friends open a link, pick a
nickname and talk: no Mumble install, no plugins. Desktop Mumble clients and browser users share the same
server, channels and voice.

```
 Browser (Svelte SPA)             Docker network
 ┌──────────────────────┐  WSS  ┌──────────────┐  TLS/TCP  ┌───────────────┐
 │ UI + AudioWorklet    │ ────► │ mumweb       │ ────────► │ mumble-server │
 │ WebCodecs Opus       │ ◄──── │ gateway (Go) │ ◄──────── │ :64738        │
 └──────────────────────┘       └──────────────┘           └───────────────┘
            ▲                          ▲
            └──── HTTPS ── Caddy ──────┘
```

## Highlights

- **Zero install for guests.** Chrome, Edge, Firefox (desktop) and Chrome on Android. Installable as a PWA.
- **A real Mumble client, not a lookalike.** The gateway opens a normal Mumble connection for every browser
  session, so the server cannot tell web users from desktop users. Voice travels over Mumble's `UDPTunnel`.
- **Lightweight gateway.** Opus is encoded and decoded in the browser (WebCodecs); the gateway never touches the
  audio. Ten simultaneous speakers use roughly 45 % of one CPU core and 15 MiB of RAM.
- **Desktop-grade controls.** Channel tree, push-to-talk or voice activation, per-user volume, the same
  right-click menus as the desktop client, channel passwords, chat, and more (see below).
- **Safe by default.** One fixed Mumble target (browsers cannot pick an address), Origin checks, per-IP and
  per-session limits, HTML-sanitised chat, SuperUser login from the web disabled unless you enable it.
- **Persistent identity.** Each browser keeps a client certificate (IndexedDB), so users can register on Mumble and
  keep their permissions. The gateway stores no certificates on disk.

### Features

| Area | What you get |
|---|---|
| Voice | Push-to-talk (rebindable key, big button on mobile) and voice activation with an adjustable threshold, mute / deafen, speaking indicators, join/leave sounds, mic and speaker selection, noise suppression toggle |
| Channels | Tree with switching (double-click), password-protected channels (lock icons, "remember in this browser"), listen to a channel, link/unlink, filter that hides empty channels, shareable `#kanal=/Path` links |
| User menu | Move, kick, ban, mute, deafen, priority speaker, local mute, ignore messages, local volume (0–200 %), nickname, view/reset comment, remove avatar, private message, info, register, friends. Admin items follow your server permissions |
| Channel menu | Join, create, edit (name, description, position, user limit, password), remove, link tools, copy URL, send message (optionally to sub-channels), pin/hide when filtering |
| Chat | Channel and private messages, HTML sanitised with DOMPurify, ignore list |
| Identity | Export / import PEM certificates, reset identity, auto-reconnect |
| UI | Dark and light themes, responsive layout with tabs on phones, installable PWA |

> The web interface is currently in Turkish. Multi-language support is on the [roadmap](docs/TODO.md).

### Browser support

| Browser | Status |
|---|---|
| Chrome / Edge (desktop) | Supported |
| Firefox (desktop) | Supported |
| Chrome (Android) | Supported |
| Safari 26+ (macOS / iOS) | Should work (WebCodecs Opus), not yet tested on a device |
| Older Safari / iOS | Not supported (no WebCodecs Opus; a WASM fallback is not planned) |

Microphone access and WebCodecs require a secure context, so the page must be served over HTTPS (or `localhost`).

> **Just want to try it on your own computer?** Follow [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): a ready-made
> `https://mumweb.test` setup with mkcert for Windows, macOS and Linux.

## Quick start (production)

Requirements: a Linux server with Docker and Docker Compose, ports 80 and 443 (TCP) and 64738 (TCP + UDP) open,
and a domain name.

1. **DNS:** point an `A` (and `AAAA`, if you have IPv6) record such as `voice.example.com` at the server. Caddy
   cannot obtain a certificate until it resolves (`dig +short voice.example.com` must show the server IP).
2. **Get the code and configure it:**
   ```sh
   git clone https://github.com/aoztrk82/mumweb.git && cd mumweb/deploy
   cp .env.example .env
   # edit .env: set MUMWEB_DOMAIN and MUMBLE_SUPERUSER_PASSWORD
   ```
3. **Start it:**
   ```sh
   docker compose up -d --build
   docker compose ps               # mumweb should become "healthy"
   docker compose logs -f caddy    # watch the certificate being issued on first start
   ```
4. Open `https://voice.example.com`. Desktop Mumble clients connect to `voice.example.com:64738`.

The compose file runs three services: `mumble` (the official `mumblevoip/mumble-server`), `mumweb` (gateway plus
web UI, non-root user, `/healthz` health check) and `caddy` (automatic HTTPS, passes WebSockets through).

## Deployment

### Mumble autoban (important)

All web users reach Mumble from the gateway's **single IP**. Mumble's default autoban (10 connection attempts in
120 s, successful ones included, then a 5-minute ban) would lock out *every* web user after a few logins. The
compose files therefore set `autobanSuccessfulConnections=false`, so only failed attempts count. Failed attempts
(wrong password) are still counted against the gateway's IP, and the gateway itself allows 10 new connections per
IP per minute.

### Banning and web users (important)

Mumble's own "Ban" also bans the user's **IP address**. Web users connect from the gateway's IP, so **banning a
web user from the desktop client bans all web users.** The "Ban…" action in the mumweb UI avoids this: it bans
only the certificate (a ban entry with the placeholder address `100::/128`, which matches no client). If a desktop
ban hits the gateway IP by mistake, delete that entry under *Server → Ban list* in the desktop client. A
certificate ban can be evaded by creating a new certificate (clearing the browser data).

Mumble also rate-limits each user (`messagelimit` = 1/s, `messageburst` = 5; covers channel joins, mute, channel
creation/ACL edits and chat) and **silently drops** the excess. If a rapid series of actions did not all happen,
wait a few seconds and retry; the desktop client has the same limit.

The `SuperUser` account cannot speak in Mumble (by design; it is for administration). SuperUser login from the web
is controlled by `MUMWEB_ALLOW_SUPERUSER` and is off in production.

### Behind an existing reverse proxy (Caddy, nginx, Traefik…)

Remove the `caddy` service, attach `mumweb` to a network the proxy can reach, and add a site to the proxy. It must
pass WebSocket upgrades on `/ws`. Keep `MUMWEB_TRUST_PROXY=true` (the proxy must add `X-Forwarded-For`) and set
`MUMWEB_ALLOWED_ORIGINS` to the exact site address.

Caddy:
```
voice.example.com {
	encode zstd gzip
	reverse_proxy mumweb:8080
}
```

nginx:
```nginx
location / {
    proxy_pass http://mumweb:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_read_timeout 120s;
}
```

### With an existing Mumble server

Remove the `mumble` service and point `MUMWEB_MUMBLE_ADDR` at the existing server (for example
`mumble-server:64738` on the same Docker network). Apply the autoban setting there too: in the official Docker
image `MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS=false`, in `mumble-server.ini`
`autobanSuccessfulConnections=false`.

### Updating

```sh
cd mumweb && git pull
cd deploy && docker compose up -d --build
docker image prune -f   # optional: remove old images
```

To update the Mumble server: `docker compose pull mumble && docker compose up -d mumble`. User registrations and
channels live in the `mumble-data` volume. Back it up with:
`docker run --rm -v mumweb_mumble-data:/d -v "$PWD":/b alpine tar czf /b/mumble-data.tgz -C /d .`

## Configuration

### Gateway environment variables

| Variable | Default | Description |
|---|---|---|
| `MUMWEB_LISTEN` | `:8080` | HTTP listen address |
| `MUMWEB_MUMBLE_ADDR` | `mumble:64738` | Target Mumble server (single and fixed) |
| `MUMWEB_TLS_VERIFY` | `false` | Verify the Mumble server certificate (leave off for self-signed) |
| `MUMWEB_TLS_FINGERPRINT` | empty | If set, pin the server certificate's SHA-256 fingerprint |
| `MUMWEB_MAX_SESSIONS` | `50` | Maximum concurrent web sessions |
| `MUMWEB_ALLOWED_ORIGINS` | empty | Comma-separated allowed `Origin`s (e.g. `https://voice.example.com`; the default port may be included). Empty means same host only |
| `MUMWEB_CONN_RATE` | `10` | New connections per IP per minute (`0` = off) |
| `MUMWEB_TRUST_PROXY` | `false` | Take the client IP from `X-Forwarded-For` (set `true` behind a reverse proxy) |
| `MUMWEB_ALLOW_SUPERUSER` | `false` | Allow logging in as Mumble's `SuperUser` from the web (when off the name is rejected; administer from the desktop client) |
| `MUMWEB_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Compose variables (`deploy/.env`)

| Variable | Default | Description |
|---|---|---|
| `MUMWEB_DOMAIN` | required | Public domain served by Caddy |
| `MUMBLE_SUPERUSER_PASSWORD` | required | Password of Mumble's `SuperUser` |
| `MUMBLE_WELCOME_TEXT` | `Welcome to our Mumble server!` | Welcome message shown on connect |
| `MUMBLE_MAX_USERS` | `50` | Mumble's own user limit |
| `MUMWEB_MAX_SESSIONS` | `50` | See above |
| `MUMWEB_LOG_LEVEL` | `info` | See above |
| `MUMWEB_ALLOW_SUPERUSER` | `false` | See above |

### Pinning the Mumble certificate

`mumprobe` prints the server fingerprint (after `make gateway`):
`./bin/mumprobe -addr <server>:64738 -exit` → `server certificate SHA-256: AB:CD:…`. Paste that value into
`MUMWEB_TLS_FINGERPRINT` as is. The fingerprint changes when the server certificate is renewed.

## Building from source

Requirements: Go 1.27+, Node 24 LTS, Docker with Compose, GNU Make. `protoc` and `protoc-gen-go` are needed only
for `make proto`.

```sh
make gateway   # bin/mumweb and bin/mumprobe, web UI embedded
make docker    # production image (mumweb:dev)
make web       # only the web UI, into gateway/web/dist
make proto     # regenerate Go code from gateway/proto
```

For day-to-day development:

```sh
cp deploy/.env.example deploy/.env   # set MUMBLE_SUPERUSER_PASSWORD; MUMBLE_PORT if 64738 is taken
make dev                             # dev Mumble server + gateway on http://localhost:8080
cd web && npm run dev                # Vite on http://localhost:5173, /ws proxied to the gateway
```

`localhost` counts as a secure context, so the microphone works over plain HTTP there. Use HTTPS when testing from
another machine ([docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)).

## Testing

```sh
make test                                                           # Go unit tests + Vitest
cd gateway && MUMWEB_TEST_MUMBLE_ADDR=localhost:64738 go test ./internal/bridge   # integration, needs a Mumble server (`make dev`)
```

- **Go:** message framing, partial `UserState` / `ChannelState` merging, `MumbleUDP.Audio` encoding, a fake TLS
  Mumble server for the client (handshake, unknown message types, reject, timeouts, fingerprint pinning, shutdown),
  certificate generation, bridge binary frames, Origin / session / IP limits. Integration: WebSocket ↔ Mumble
  text, audio, state, certificate persistence, admin actions, channel passwords, name-takeover protection, goroutine leaks.
- **Vitest:** binary audio frames, jitter buffer, framing and resampling, voice activation, bitrate, PEM handling,
  reconnect backoff.
- **Manual acceptance checklist:** [docs/TESTING.md](docs/TESTING.md).

`bin/mumprobe` is a CLI that talks to a Mumble server directly: it prints the channel tree, logs events and audio
packets, and can send text (`-say`), play an Opus packet file (`-play`), transmit silence (`-tx`) and create a
channel (`-mkchannel`).

## Repository layout

| Path | Contents |
|---|---|
| `gateway/cmd/mumweb` | HTTP / WebSocket gateway (serves the UI, `/ws`, `/healthz`) |
| `gateway/cmd/mumprobe` | Mumble probing CLI |
| `gateway/internal/mumble` | Mumble TCP client library (TLS, framing, state, audio tunnel) |
| `gateway/internal/bridge` | WebSocket session ↔ Mumble client |
| `gateway/internal/certs` | Client certificate generation |
| `gateway/proto` | Official `Mumble.proto` / `MumbleUDP.proto` and generated Go code |
| `web/src/lib` | WebSocket client, audio pipeline (`audio/`), state stores |
| `web/src/components` | UI components |
| `deploy/` | Production compose, Caddyfile, `.env.example`, dev compose |
| `deploy/local/` | Local HTTPS test environment (mkcert) |
| `docs/` | Development guide, testing checklist, progress log, TODO |

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| Site does not open; ACME errors in the Caddy log | DNS does not point to the server yet, or ports 80/443 are closed or in use. `docker compose logs caddy` |
| "Cannot reach the Mumble server" | `docker compose logs mumble mumweb`; is Mumble running, is `MUMWEB_MUMBLE_ADDR` correct? |
| After a few logins nobody can connect | Mumble autoban banned the gateway IP: apply the autoban setting above, restart Mumble to clear the ban |
| Endless "Reconnecting" and `ws origin rejected` in the gateway log | `MUMWEB_ALLOWED_ORIGINS` must match the address in the browser exactly (with `https://`, no trailing `/`) |
| Everyone appears to come from the same IP / quick "too many connections" | Behind a reverse proxy set `MUMWEB_TRUST_PROXY=true`, and make the proxy send `X-Forwarded-For` |
| No microphone prompt | The page must be served over HTTPS (microphone and WebCodecs need a secure context) |
| "Your browser does not support Opus encoding" | Safari / iOS or an old browser: use a current Chrome, Edge or Firefox |
| The other side hears an echo without headphones | Browser echo cancellation cannot always capture WebAudio output on every platform: use headphones or push-to-talk |
| "This nickname is registered; wrong password or certificate" | The name is registered in Mumble with another certificate. Use the right browser or import the certificate in Settings → Identity |
| Certificate lost (browser data cleared) | Import the previously exported PEM in Settings → Identity → Import; otherwise an admin must move the registration to the new certificate |

Gateway logs are JSON: `docker compose logs -f mumweb`. Passwords, certificate keys and chat content are never logged.

## Documentation

| | English | Türkçe |
|---|---|---|
| Overview and deployment | this file | [README.tr.md](README.tr.md) |
| Local HTTPS test environment | [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | [docs/tr/DEVELOPMENT.md](docs/tr/DEVELOPMENT.md) |
| Manual acceptance tests | [docs/TESTING.md](docs/TESTING.md) | [docs/tr/TESTING.md](docs/tr/TESTING.md) |
| Progress log and findings | [docs/PROGRESS.md](docs/PROGRESS.md) | [docs/tr/PROGRESS.md](docs/tr/PROGRESS.md) |
| Roadmap / TODO | [docs/TODO.md](docs/TODO.md) | [docs/tr/TODO.md](docs/tr/TODO.md) |

## License

[MIT](LICENSE)
