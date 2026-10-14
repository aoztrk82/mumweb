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
- **Admin panel.** An optional `/admin/` page (off until you set a password) with an ACL and group editor, ban list,
  registered users and channel management; it runs next to the voice client in the same browser.
- **Safe by default.** One fixed Mumble target (browsers cannot pick an address), Origin checks, per-IP and
  per-session limits, a strict Content-Security-Policy, HTML-sanitised chat, SuperUser login from the web disabled unless
  you enable it, and a hardened compose file (read-only gateway without network egress). See
  [docs/SECURITY.md](docs/SECURITY.md).
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
| UI | English / Turkish interface with a one-click switch, dark and light themes, responsive layout with tabs on phones, installable PWA |

> The interface is available in **English and Turkish**. It follows the browser language (Turkish when the browser
> language is neither), and an EN / TR switch is always visible on the login screen and in the status bar.

### Browser support

| Browser | Status |
|---|---|
| Chrome / Edge (desktop) | Supported |
| Firefox (desktop) | Supported |
| Samsung Internet (Android) | Works (voice and chat tested with a desktop client) |
| Chrome (Android) | Should work, not yet tested on a device |
| Firefox (Android) | Not working yet: reports that the codec is not supported (under investigation) |
| Safari 26+ (macOS / iOS) | Should work (WebCodecs Opus), not yet tested on a device |
| Older Safari / iOS | Not supported (no WebCodecs Opus; a WASM fallback is not planned) |

Microphone access and WebCodecs require a secure context, so the page must be served over HTTPS (or `localhost`).

Known limit on Android: when the browser is sent to the background, the session drops out of the room after about
15–20 seconds (a locked screen is fine). See the [roadmap](docs/TODO.md).

> **Just want to try it on your own computer?** Follow [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md): a ready-made
> `https://mumweb.test` setup with mkcert for Windows, macOS and Linux.

## Deployment

There is nothing to build: the ready-made image is [`aoztrk82/mumweb`](https://hub.docker.com/r/aoztrk82/mumweb) on
Docker Hub, for `linux/amd64` and `linux/arm64`. You need a Linux server with Docker and Compose, and a domain name.

### How the pieces fit

| Service | Needed? | Purpose |
|---|---|---|
| `mumweb` | Always | The gateway and web UI. One instance talks to exactly one Mumble server. |
| `mumble` (official `mumblevoip/mumble-server`) | Only if you don't already run a Mumble server | The voice server; desktop Mumble clients connect to it directly on `64738`. |
| A reverse proxy with TLS (Caddy, nginx, Traefik…) | Always | Browsers need HTTPS for microphone access. It only needs to pass `/ws` through as a WebSocket. |

**If you already have a server** — most self-hosters running a homelab or a VPS already have a compose file with a
reverse proxy, and often a Mumble server too — add `mumweb` to what you already run (**Option A**). **Starting from a
bare server**, our compose file brings up all three together (**Option B**).

### Option A: add mumweb to your existing setup

Add `mumweb` as one more service to your own `docker-compose.yml`, on whatever network your reverse proxy (and
Mumble, if you have one) already use:

```yaml
services:
  mumweb:
    image: aoztrk82/mumweb:${MUMWEB_VERSION:-latest}
    restart: unless-stopped
    environment:
      MUMWEB_MUMBLE_ADDR: mumble:64738                  # host:port of your Mumble server
      MUMWEB_ALLOWED_ORIGINS: https://voice.example.com  # exact address browsers will use, with scheme
      MUMWEB_TRUST_PROXY: "true"                         # your proxy must add X-Forwarded-For
      MUMWEB_MAX_SESSIONS: "50"
    networks: [proxy, mumble]   # whatever networks your proxy and Mumble container already use
```

That's it for the gateway: everything else has a sane default, see
[gateway environment variables](#gateway-environment-variables) for the full list. If you don't already run a
Mumble server, copy the `mumble` service from [`deploy/docker-compose.yml`](deploy/docker-compose.yml) into your
file as well, and read [Mumble autoban](#mumble-autoban-important) below — it matters from the first login.

Then point your reverse proxy at `mumweb:8080` (the container's internal port) and make sure it passes WebSocket
upgrades on `/ws`:

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

Traefik (compose labels on the `mumweb` service):
```yaml
    labels:
      - traefik.enable=true
      - traefik.http.routers.mumweb.rule=Host(`voice.example.com`)
      - traefik.http.routers.mumweb.tls.certresolver=letsencrypt
      - traefik.http.services.mumweb.loadbalancer.server.port=8080
```

`docker compose up -d` and open your domain.

### Option B: starting from scratch (use our compose)

No existing server to attach to? This brings up Mumble, mumweb and Caddy (automatic HTTPS) together. The same files
live in [`deploy/`](deploy/) in this repository (with a commented `.env.example`).

1. **DNS:** point an `A` (and `AAAA`, if you have IPv6) record such as `voice.example.com` at the server. Caddy
   cannot obtain a certificate until it resolves (`dig +short voice.example.com` must show the server IP).
2. **Create a folder with three files:**
   ```sh
   mkdir mumweb && cd mumweb
   ```
   `docker-compose.yml`:
   ```yaml
   # Production stack: Mumble server + mumweb gateway + Caddy (automatic HTTPS).
   # Put MUMWEB_DOMAIN and MUMBLE_SUPERUSER_PASSWORD in a .env file next to this one, then:
   #   docker compose up -d
   name: mumweb

   services:
     mumble:
       image: mumblevoip/mumble-server:latest
       restart: unless-stopped
       ports:
         # Desktop Mumble clients connect here directly.
         - "64738:64738/tcp"
         - "64738:64738/udp"
       volumes:
         - mumble-data:/data
       environment:
         MUMBLE_SUPERUSER_PASSWORD: ${MUMBLE_SUPERUSER_PASSWORD:?set MUMBLE_SUPERUSER_PASSWORD in .env}
         MUMBLE_CONFIG_WELCOMETEXT: ${MUMBLE_WELCOME_TEXT:-Welcome to our Mumble server!}
         MUMBLE_CONFIG_USERS: ${MUMBLE_MAX_USERS:-50}
         # All web users reach Mumble from the gateway's single IP. The default autoban
         # (10 connections / 120 s, successful ones included) would lock them all out
         # for 5 minutes after a few logins. Count only failed attempts.
         MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS: "false"
       # Hardening: no extra privileges. (cap_drop: ALL breaks the image's entrypoint, which sets
       # capabilities before it drops to its own user.) Reachable by the gateway on the internal network.
       security_opt: ["no-new-privileges:true"]
       pids_limit: 512
       networks: [default, backend]

     mumweb:
       image: aoztrk82/mumweb:${MUMWEB_VERSION:-latest}
       restart: unless-stopped
       environment:
         MUMWEB_MUMBLE_ADDR: mumble:64738
         MUMWEB_ALLOWED_ORIGINS: https://${MUMWEB_DOMAIN:?set MUMWEB_DOMAIN in .env}
         # Behind Caddy: take the client IP from X-Forwarded-For for per-IP limits.
         MUMWEB_TRUST_PROXY: "true"
         MUMWEB_MAX_SESSIONS: ${MUMWEB_MAX_SESSIONS:-50}
         MUMWEB_LOG_LEVEL: ${MUMWEB_LOG_LEVEL:-info}
         # Logging in as Mumble's SuperUser from the browser. Off by default: the web login
         # is reachable by anyone who knows the URL; use the desktop client for admin work.
         MUMWEB_ALLOW_SUPERUSER: ${MUMWEB_ALLOW_SUPERUSER:-false}
         # Optional password every web user must enter (independent of the Mumble server password).
         MUMWEB_ACCESS_PASSWORD: ${MUMWEB_ACCESS_PASSWORD:-}
         # Switches on the /admin panel (empty = panel off). Separate from the access password.
         MUMWEB_ADMIN_PASSWORD: ${MUMWEB_ADMIN_PASSWORD:-}
         # Keep some of the MUMWEB_MAX_SESSIONS slots for people who know this password (off if either is empty).
         MUMWEB_RESERVED_SLOTS: ${MUMWEB_RESERVED_SLOTS:-0}
         MUMWEB_RESERVED_PASSWORD: ${MUMWEB_RESERVED_PASSWORD:-}
       # Hardening: nothing writable, no capabilities, resource limits, and no way out to the
       # internet: the gateway only sits on the internal network with Mumble and Caddy.
       read_only: true
       cap_drop: [ALL]
       security_opt: ["no-new-privileges:true"]
       mem_limit: 256m
       pids_limit: 200
       cpus: 1.0
       networks: [backend]
       depends_on:
         - mumble

     caddy:
       image: caddy:2
       restart: unless-stopped
       ports:
         - "80:80"
         - "443:443"
         - "443:443/udp"
       environment:
         MUMWEB_DOMAIN: ${MUMWEB_DOMAIN}
       volumes:
         - ./Caddyfile:/etc/caddy/Caddyfile:ro
         - caddy-data:/data
         - caddy-config:/config
       cap_drop: [ALL]
       cap_add: [NET_BIND_SERVICE]
       security_opt: ["no-new-privileges:true"]
       pids_limit: 200
       networks: [default, backend]
       depends_on:
         - mumweb

   networks:
     # No route to the outside: only Mumble, the gateway and Caddy talk here.
     backend:
       internal: true

   volumes:
     mumble-data:
     caddy-data:
     caddy-config:
   ```
   `Caddyfile`:
   ```
   {$MUMWEB_DOMAIN} {
   	encode zstd gzip
   	# Caddy proxies WebSocket upgrades (/ws) transparently.
   	reverse_proxy mumweb:8080
   }
   ```
   `.env`:
   ```sh
   MUMWEB_DOMAIN=voice.example.com
   MUMBLE_SUPERUSER_PASSWORD=change-me
   ```
3. **Start it:**
   ```sh
   docker compose up -d
   docker compose ps               # mumweb should become "healthy"
   docker compose logs -f caddy    # watch the certificate being issued on first start
   ```
4. Open `https://voice.example.com`. Desktop Mumble clients connect to `voice.example.com:64738`.

The compose file runs three services: `mumble` (the official `mumblevoip/mumble-server`), `mumweb` (gateway plus
web UI, non-root user, `/healthz` health check) and `caddy` (automatic HTTPS, passes WebSockets through).

Image tags: `latest` follows the newest release; `0.3.1`-style tags are fixed releases. Pin one by setting
`MUMWEB_VERSION=0.3.1` in `.env`. `docker compose pull` then only fetches that version, never a newer one, until you
change it on purpose.

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

The `SuperUser` account cannot speak in Mumble (by design; it is for administration). SuperUser login from the main
web client is controlled by `MUMWEB_ALLOW_SUPERUSER` and is off in production; for administration use the
[admin panel](#admin-panel-admin) (`MUMWEB_ADMIN_PASSWORD`) or the desktop client.

If you went with Option A and already run Mumble elsewhere, just point `MUMWEB_MUMBLE_ADDR` at it (for example
`mumble-server:64738` on the same Docker network) and apply the autoban setting there too: in the official Docker
image `MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS=false`, in `mumble-server.ini`
`autobanSuccessfulConnections=false`.

### Updating

```sh
docker compose pull          # fetch the newest mumweb (and Mumble) images
docker compose up -d         # recreate only what changed
docker image prune -f        # optional: remove old images
```

User registrations and
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
| `MUMWEB_MAX_SESSIONS_PER_IP` | `10` | Maximum concurrent web sessions per client IP (`0` = off). People behind one NAT share it; behind a proxy set `MUMWEB_TRUST_PROXY` |
| `MUMWEB_ALLOWED_ORIGINS` | empty | Comma-separated allowed `Origin`s (e.g. `https://voice.example.com`; the default port may be included). Empty means same host only |
| `MUMWEB_CONN_RATE` | `10` | New connections per IP per minute (`0` = off) |
| `MUMWEB_TRUST_PROXY` | `false` | Take the client IP from `X-Forwarded-For` (set `true` behind a reverse proxy) |
| `MUMWEB_ALLOW_SUPERUSER` | `false` | Allow logging in as Mumble's `SuperUser` from the web (when off the name is rejected in the main client; administer with the admin panel or the desktop client) |
| `MUMWEB_ACCESS_PASSWORD` | empty | If set, every web user must enter this password on the login screen. Separate from the Mumble server password and only enforced by the gateway (see below) |
| `MUMWEB_ADMIN_PASSWORD` | empty | If set, switches on the admin panel at `/admin`; admin sessions must send it. Empty = panel off (see below) |
| `MUMWEB_RESERVED_SLOTS` | `0` | How many of `MUMWEB_MAX_SESSIONS` are reserved for holders of `MUMWEB_RESERVED_PASSWORD` (and for admin sessions). Other users share the remaining slots. Needs the password and a limit above the reserved count, else off |
| `MUMWEB_RESERVED_PASSWORD` | empty | Password for the reserved slots, typed in an optional login field |
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
| `MUMWEB_ACCESS_PASSWORD` | empty | See above |
| `MUMWEB_ADMIN_PASSWORD` | empty | See above |
| `MUMWEB_RESERVED_SLOTS`, `MUMWEB_RESERVED_PASSWORD` | `0`, empty | See above |

### Web access password

Set `MUMWEB_ACCESS_PASSWORD` to keep strangers out of the web client. The login screen then shows an extra
"Access password" field (with a "remember in this browser" option); it appears only when the feature is on.

- It is **not** the Mumble server password. Desktop Mumble clients connect straight to port 64738 and never see it,
  so protect the server itself with Mumble's own password if you need that too.
- The gateway checks it before it connects to Mumble, compares in constant time, waits a second after a wrong
  guess, and locks an IP out for 10 minutes after 5 wrong guesses.
- It travels over the same HTTPS/WSS connection as everything else, so use HTTPS. Treat it as a shared "door code",
  not as per-user authentication.

### Reserved web slots

`MUMWEB_MAX_SESSIONS` limits how many people use the web client at once, whatever the Mumble server allows (for
example `MUMWEB_MAX_SESSIONS=10` on a 50 slot server). To keep some of them for yourself and friends, like an admin
slot in a game server, set `MUMWEB_RESERVED_SLOTS=2` and `MUMWEB_RESERVED_PASSWORD`. With 10 web slots, everyone shares
8; people who enter the reserved password in the optional "Reserved slot password" field of the login screen can also
use the last 2. The field appears only when the feature is on.

- Normal users who find the 8 slots taken see "All web slots are taken" and can retry with the password.
- Admin panel sessions count as privileged too, so you can always get in to manage the server.
- A wrong reserved password is refused (same delay and 10 minute lockout as the access password, counted separately),
  so a typo does not silently use up a general slot.
- It is a shared "door code" like the access password: use HTTPS. Desktop Mumble clients are not affected.

### Admin panel (`/admin`)

Set `MUMWEB_ADMIN_PASSWORD` to switch on the admin panel at `https://your-domain/admin/`. Without it the panel is
off and its login is refused. It manages what the Mumble protocol exposes:

- **Online:** kick, ban (certificate only), server mute/deafen, register.
- **Channels:** tree with create / rename / remove, the channel password, and a full **ACL and group editor**
  (rules and their order, allow/deny per permission, inherit flags, group members).
- **Bans:** the ban list. You can remove any entry (also IP bans made from the desktop client) and add certificate bans.
- **Registered users:** list, rename, delete.

Server settings (welcome text, bandwidth, user limit) are not part of the Mumble protocol; change them in the
server's configuration.

- **Log in with a separate account.** The panel asks for the admin password, then for a Mumble account, usually
  `SuperUser` with the Mumble admin password. SuperUser works here even though `MUMWEB_ALLOW_SUPERUSER` is `false` for
  the main client, so you can keep the main login closed. Registered users with the right permissions (a group
  with Ban / Register / Write) work too.
- **Voice and admin at the same time, same browser.** The panel keeps its own certificate (identity) and carries no
  audio, so a second tab can run the voice client while `/admin` is open. Use two different accounts: Mumble drops the
  older session when the same registered account connects twice.
- The admin password replaces the web access password for this page. The gateway checks it before it connects to
  Mumble, with its own brute-force lockout (5 wrong guesses lock the IP for 10 minutes, separate from the access
  password's lockout). The Mumble server still enforces permissions for every action.
- Every state-changing admin action is logged (action, ids, counts; never passwords, chat or ACL group names, because
  a `#name` group is a channel password). Writes are paced to stay under Mumble's message rate limit.
- It needs HTTPS in production (the password travels over the WebSocket). Keep the value long and random. If you do not
  want the page reachable at all, block `/admin*` in your reverse proxy.

### Pinning the Mumble certificate

`mumprobe` prints the server fingerprint (after `make gateway`):
`./bin/mumprobe -addr <server>:64738 -exit` → `server certificate SHA-256: AB:CD:…`. Paste that value into
`MUMWEB_TLS_FINGERPRINT` as is. The fingerprint changes when the server certificate is renewed.

## Building from source

Most people should use the Docker Hub image above. To build it yourself, hack on the code or run the tests, see
**[docs/BUILDING.md](docs/BUILDING.md)**.

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
| Docker Hub page text | [docs/DOCKERHUB.md](docs/DOCKERHUB.md) | (English only) |
| Building from source, testing, publishing the image | [docs/BUILDING.md](docs/BUILDING.md) | [docs/tr/BUILDING.md](docs/tr/BUILDING.md) |
| Local HTTPS test environment | [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) | [docs/tr/DEVELOPMENT.md](docs/tr/DEVELOPMENT.md) |
| Manual acceptance tests | [docs/TESTING.md](docs/TESTING.md) | [docs/tr/TESTING.md](docs/tr/TESTING.md) |
| Progress log and findings | [docs/PROGRESS.md](docs/PROGRESS.md) | [docs/tr/PROGRESS.md](docs/tr/PROGRESS.md) |
| Roadmap / TODO | [docs/TODO.md](docs/TODO.md) | [docs/tr/TODO.md](docs/tr/TODO.md) |
| Security review, threat model, hardening | [docs/SECURITY.md](docs/SECURITY.md) (policy: [SECURITY.md](SECURITY.md)) | [docs/tr/SECURITY.md](docs/tr/SECURITY.md) |
| Short-lived test deployment on a server | [docs/TEST-DEPLOYMENT.md](docs/TEST-DEPLOYMENT.md) | [docs/tr/TEST-DEPLOYMENT.md](docs/tr/TEST-DEPLOYMENT.md) |

## License

[MIT](LICENSE)
