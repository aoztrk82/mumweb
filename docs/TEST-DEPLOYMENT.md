# Test deployment (1-2 days)

**English** | [Türkçe](tr/TEST-DEPLOYMENT.md)

A short-lived public instance used for stress and security testing before the real production setup. It runs the
production stack with looser limits (`deploy/test/`). This page is written so that a Claude Code session (or a person)
on the server can set it up without any other context.

## What you are setting up

mumweb = a Mumble server + a Go gateway + Caddy (automatic HTTPS), all in Docker Compose:

```
Browser ⇄ HTTPS (Caddy :443) ⇄ gateway (internal network only) ⇄ Mumble server (:64738)
```

The images come from Docker Hub (`aoztrk82/mumweb`, `mumblevoip/mumble-server`, `caddy`); nothing needs to be built
on the server. The test site will be up for 1-2 days and then replaced by the real production configuration.

## What differs from production

| Setting | Production file (`deploy/docker-compose.yml`) | Test file (`deploy/test/compose.yaml`) |
|---|---|---|
| Web sessions (`MUMWEB_MAX_SESSIONS`) | 50 | 200 |
| Sessions per IP (`MUMWEB_MAX_SESSIONS_PER_IP`) | 10 (gateway default) | 100 |
| New connections per IP per minute (`MUMWEB_CONN_RATE`) | 10 (gateway default) | 600 |
| Mumble user limit | 50 | 200 |
| Gateway log level | info | debug |
| Gateway memory / CPU / processes | 256 MiB / 1 / 200 | 512 MiB / 2 / 400 |
| Everything else (read-only gateway, no egress, capabilities dropped, HSTS, CSP) | same | same |

The loose per-IP numbers exist because the tests come from one machine. Do **not** keep them in production.

## Prerequisites (ask the owner for what is missing)

1. A Linux server with Docker Engine and the Compose plugin (`docker compose version`), and `git`.
2. A domain name for the test site with a DNS A/AAAA record pointing at the server's public IP.
3. Open ports in the host firewall and the cloud firewall: **80/tcp, 443/tcp, 443/udp** (HTTPS), **64738/tcp and
   64738/udp** (desktop Mumble clients, optional for web-only tests).
4. The values for `.env` (below). Ask the owner; never invent passwords and never commit them.

## Steps

```sh
git clone https://github.com/aoztrk82/mumweb.git
cd mumweb/deploy/test
cp .env.example .env        # edit: domain and the passwords (chmod 600 .env)
docker compose pull
docker compose up -d
docker compose ps           # three services running
curl -s https://YOUR-DOMAIN/healthz     # ok
curl -s https://YOUR-DOMAIN/config      # {"accessPassword":true,"admin":true,"reservedSlots":true}
```

`.env` values: `MUMWEB_DOMAIN`, `MUMBLE_SUPERUSER_PASSWORD`, `MUMWEB_ACCESS_PASSWORD`, `MUMWEB_ADMIN_PASSWORD`,
optionally `MUMWEB_RESERVED_SLOTS` + `MUMWEB_RESERVED_PASSWORD`. The file is git-ignored. Caddy fetches the TLS
certificate on first start; it needs ports 80 and 443 reachable and DNS already pointing at the server (watch
`docker compose logs caddy`).

Reserved slots need image `0.3.1` or newer; with `latest` at an older release the variables are ignored and `/config`
shows `"reservedSlots":false`. Tell the owner if that happens.

## Checks after it is up

1. Open `https://YOUR-DOMAIN/`, enter the access password and a nickname: you land in the Mumble channel tree.
2. `https://YOUR-DOMAIN/admin/` asks for the admin password and the SuperUser password; the tabs load.
3. Response headers contain `Content-Security-Policy` and `Strict-Transport-Security`:
   `curl -sI https://YOUR-DOMAIN/ | grep -i -E 'content-security|strict-transport'`.
4. The gateway cannot reach the internet: `docker compose exec mumweb wget -T 4 -O- http://1.1.1.1` fails;
   `docker compose exec mumweb touch /x` fails (read-only).

## Operating it

| Task | Command (in `deploy/test`) |
|---|---|
| Logs (gateway) | `docker compose logs -f --tail=200 mumweb` |
| Logs (all) | `docker compose logs --since 1h > /tmp/mumweb-logs.txt` |
| Resource use | `docker stats --no-stream` |
| Update to the newest image | `docker compose pull && docker compose up -d` |
| Pin a version | set `MUMWEB_VERSION=x.y.z` in `.env`, then `docker compose up -d` |
| Stop, keep data | `docker compose down` |
| Remove everything including Mumble data and certificates | `docker compose down -v` |

The gateway logs user names, IPs and events, never passwords, certificate keys or chat text. When reporting a problem,
send the relevant log lines, the `docker compose ps` output and `docker stats` numbers; do not paste `.env`.

## Rules for whoever runs this

- Do not change the hardening (`read_only`, `cap_drop`, the `backend` internal network) to "fix" something; report it
  instead, it is part of what is being tested.
- Do not publish the gateway port (8080) or Mumble's Ice port; only Caddy (80/443) and Mumble (64738) are exposed.
- The stress and security tests are run by the owner's other session against this URL. Expect many connections from one
  IP, failed password attempts and lockouts in the logs; that is the test, not an attack. Anything else unusual: report it.
- Do not edit files in the repository on the server; the owner changes code elsewhere and you `git pull` + `docker
  compose pull`.

## Moving to real production

After the tests the owner decides the final limits. Then switch to `deploy/docker-compose.yml` (production defaults,
from the README quick start), a fresh `deploy/.env` (see `deploy/.env.example`), pinned image versions, and, if wanted,
the Caddy rule that restricts `/admin` to one address ([SECURITY.md](SECURITY.md) section 5). Run
`docker compose -f deploy/test/compose.yaml down` (add `-v` to wipe test data) before starting the production stack on
the same ports.
