# mumweb

**A self-hosted web client for [Mumble](https://www.mumble.info/) voice chat.** Friends open a link, pick a nickname and
talk, with no Mumble install. Desktop Mumble clients and browser users share the same server, channels and voice.

- Source code, full documentation, issues: **https://github.com/aoztrk82/mumweb**
- Full setup guide: https://github.com/aoztrk82/mumweb#quick-start-production
- Türkçe: https://github.com/aoztrk82/mumweb/blob/main/README.tr.md

## What is in this image

The mumweb **gateway** (a small Go binary) with the **web UI** embedded. For every browser session it opens a normal
Mumble connection and bridges it over a WebSocket; Opus audio is encoded and decoded in the browser, so the gateway is
light on CPU and RAM. It runs as a non-root user and has a `/healthz` health check.

It does **not** include a Mumble server or HTTPS: run it next to `mumblevoip/mumble-server` and a reverse proxy such as
Caddy (the compose file below does exactly that).

## Tags

| Tag | Meaning |
|---|---|
| `latest` | the newest release |
| `0.3.0`, … | fixed releases (pin one for reproducible installs) |

Platforms: `linux/amd64`, `linux/arm64`.

## Quick start

The full, ready-to-copy `docker-compose.yml` (Mumble server + mumweb + Caddy with automatic HTTPS), the `Caddyfile` and
the `.env` are in the README on GitHub:
**https://github.com/aoztrk82/mumweb#quick-start-production**

In short: point a DNS record at your server, put those three files in a folder, set `MUMWEB_DOMAIN` and
`MUMBLE_SUPERUSER_PASSWORD` in `.env`, and run `docker compose up -d`.

Only the mumweb service, if you already have a Mumble server and a reverse proxy:

```yaml
services:
  mumweb:
    image: aoztrk82/mumweb:latest
    restart: unless-stopped
    environment:
      MUMWEB_MUMBLE_ADDR: mumble:64738          # your Mumble server
      MUMWEB_ALLOWED_ORIGINS: https://voice.example.com
      MUMWEB_TRUST_PROXY: "true"                # behind a reverse proxy
    # the reverse proxy must pass WebSocket upgrades on /ws to port 8080
```

Update with `docker compose pull && docker compose up -d`.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `MUMWEB_LISTEN` | `:8080` | HTTP listen address |
| `MUMWEB_MUMBLE_ADDR` | `mumble:64738` | The Mumble server to connect to (fixed; browsers cannot choose) |
| `MUMWEB_ALLOWED_ORIGINS` | empty | Comma-separated allowed `Origin`s, e.g. `https://voice.example.com` |
| `MUMWEB_TRUST_PROXY` | `false` | Take the client IP from `X-Forwarded-For` (set `true` behind a proxy) |
| `MUMWEB_ACCESS_PASSWORD` | empty | If set, web users must enter this password (separate from the Mumble server password) |
| `MUMWEB_ADMIN_PASSWORD` | empty | If set, switches on the admin panel at `/admin` (ban list, ACLs, registered users); empty = off |
| `MUMWEB_MAX_SESSIONS` | `50` | Maximum concurrent web sessions |
| `MUMWEB_MAX_SESSIONS_PER_IP` | `10` | Maximum concurrent web sessions per client IP (`0` = off) |
| `MUMWEB_RESERVED_SLOTS` | `0` | Of `MUMWEB_MAX_SESSIONS`, how many are kept for people who enter `MUMWEB_RESERVED_PASSWORD` (and admin sessions) |
| `MUMWEB_RESERVED_PASSWORD` | empty | Password for the reserved slots (optional login field); both must be set |
| `MUMWEB_CONN_RATE` | `10` | New connections per IP per minute (`0` = off) |
| `MUMWEB_ALLOW_SUPERUSER` | `false` | Allow logging in as Mumble's `SuperUser` from the web |
| `MUMWEB_TLS_VERIFY` | `false` | Verify the Mumble server certificate |
| `MUMWEB_TLS_FINGERPRINT` | empty | Pin the Mumble server certificate's SHA-256 fingerprint |
| `MUMWEB_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

Port `8080` serves the UI, `/admin/`, `/ws` (WebSocket), `/config` and `/healthz`.

**Important:** all web users reach Mumble from the gateway's single IP, so set
`MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS=false` on the Mumble server (the compose file on GitHub does). Details and
the ban behaviour are explained in the README.

## Features

Push-to-talk and voice activation, channel tree and switching, password-protected channels, per-user volume, the same
right-click menus as the desktop client (kick, ban, mute, move, register, channel edit/link/listen…), chat, English and
Turkish interface, dark/light theme, certificate export/import, PWA, auto-reconnect.
Works in Chrome and Edge, Firefox (desktop) and Samsung Internet; HTTPS is required for the microphone.

## License

MIT. https://github.com/aoztrk82/mumweb/blob/main/LICENSE
