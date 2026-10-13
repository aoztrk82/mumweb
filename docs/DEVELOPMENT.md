# Local test environment (step by step, Windows first)

**English** | [Türkçe](tr/DEVELOPMENT.md)

This guide runs mumweb on your own computer **with real HTTPS**. At the end you can open `https://mumweb.test` in a
browser, try voice chat, and connect to the same server with the desktop Mumble client. It is written for Windows;
macOS and Linux differences are at the end.

```
 Browser ──► https://mumweb.test ──► Caddy (mkcert certificate) ──► mumweb ──► Mumble server
                                                                                    ▲
 Desktop Mumble client ──────────────────► mumweb.test:64738 ───────────────────────┘
```

`deploy/local/compose.yaml` runs four containers:

| Service | Purpose |
|---|---|
| `certs` | One-shot: copies the mkcert files into a volume with permissions the Mumble server can read, then exits. |
| `mumble` | The official Mumble server (`mumblevoip/mumble-server`) using the mkcert certificate. Port 64738 is open for desktop clients. |
| `mumweb` | Gateway + web UI built from this repository. |
| `caddy` | Serves `https://mumweb.test` with the mkcert certificate and proxies requests to mumweb. |

> **Why HTTPS?** Browsers only allow microphone access and WebCodecs audio encoding in a secure context (HTTPS or
> `localhost`). mkcert creates a local certificate authority that your computer trusts, so the browser shows no
> warning.

## Quick start

With the tools installed it takes five steps. Details below.

```powershell
# 1) Once: install mkcert's local certificate authority
mkcert -install

# 2) Generate the certificate
cd mumweb\deploy\local
mkcert -cert-file certs\cert.pem -key-file certs\key.pem mumweb.test localhost 127.0.0.1 ::1

# 3) Add to the hosts file (in an Administrator PowerShell)
Add-Content -Path "$env:SystemRoot\System32\drivers\etc\hosts" -Value "`n127.0.0.1 mumweb.test`n::1 mumweb.test"

# 4) Start
docker compose up -d --build

# 5) Open: https://mumweb.test
```

---

## 1. Requirements

| Tool | Why | Install |
|---|---|---|
| **Docker Desktop** (WSL 2 backend) | Runs the containers | <https://www.docker.com/products/docker-desktop/> or `winget install Docker.DockerDesktop` |
| **Git** | To get the code | `winget install Git.Git` |
| **mkcert** | Local, trusted HTTPS certificate | `winget install FiloSottile.mkcert` (alternatively `choco install mkcert` / `scoop install mkcert`) |
| Mumble desktop client (optional) | Web ↔ desktop tests | <https://www.mumble.info/downloads/> |

You do **not** need Go, Node or any other build tool; everything is built inside Docker.

After installing with `winget`, open a new PowerShell window so the commands are on your `PATH`. Check that Docker
Desktop is running:

```powershell
docker version
docker compose version
mkcert -version
```

## 2. Get the code

```powershell
git clone https://github.com/aoztrk82/mumweb.git
cd mumweb
```

The repository's `.gitattributes` makes files that are read inside containers (Caddyfile, compose, Dockerfile) keep
LF line endings on Windows; no extra Git setting is needed.

## 3. Certificate with mkcert

### 3.1 Install the local certificate authority (once)

```powershell
mkcert -install
```

Windows shows a security prompt ("Do you want to install this certificate?"); answer **Yes**. This adds mkcert's root
certificate to the Windows certificate store, which Chrome and Edge use.

> **Firefox:** Current Firefox versions trust root certificates in the Windows store. If you still get a certificate
> warning, set `security.enterprise_roots.enabled` to `true` in `about:config` and restart Firefox.

### 3.2 Generate a certificate for `mumweb.test`

```powershell
cd deploy\local
mkcert -cert-file certs\cert.pem -key-file certs\key.pem mumweb.test localhost 127.0.0.1 ::1
```

The file names matter: Caddy reads `certs\cert.pem` and `certs\key.pem`. The contents of `certs` are not added to git
(the private key stays with you).

> To use a different domain (for example `voice.test`), generate the certificate for that name, put it in the hosts
> file, and set `MUMWEB_DOMAIN=voice.test` in `.env` (see [Settings](#7-settings-env)). Avoid `.local` names: Windows
> looks them up over mDNS and it is slow. `.test` is reserved for this purpose.

## 4. hosts file

The name `mumweb.test` must point to your computer.

**Option A: Notepad.** Right-click *Notepad* in the Start menu → **Run as administrator** → open
`C:\Windows\System32\drivers\etc\hosts` (file type: *All Files*) and append:

```
127.0.0.1 mumweb.test
::1 mumweb.test
```

**Option B: PowerShell (as Administrator):**

```powershell
Add-Content -Path "$env:SystemRoot\System32\drivers\etc\hosts" -Value "`n127.0.0.1 mumweb.test`n::1 mumweb.test"
```

Then flush the DNS cache and check:

```powershell
ipconfig /flushdns
ping mumweb.test     # should answer from 127.0.0.1 or ::1
```

## 5. Start

```powershell
cd deploy\local        # from the repository root
docker compose up -d --build
```

The first run downloads images and builds the web UI and gateway, which can take a few minutes. Later starts take
seconds.

Check the status:

```powershell
docker compose ps
```

`mumweb` should be `healthy`, `caddy` and `mumble` should be `Up`. It is normal for `certs` to show `Exited (0)` (it
does its job and exits; visible with `docker compose ps -a`).

## 6. Use it

### 6.1 From the browser

> The interface is currently Turkish; button names below are given in English (for example **Connect** is "Bağlan").

1. In Chrome, Edge or Firefox open **<https://mumweb.test>**. The address bar should show a lock and no warning.
2. Type a nickname and click **Connect**. The browser asks for microphone permission; allow it.
3. The channel tree, chat and the control bar at the bottom appear. **Right-click** a user to lower or mute them just
   for yourself; right-click (or double-click) a channel to join it. The default talk mode is **voice activation**;
   the level bar should cross the yellow line while you speak. In **push-to-talk** mode the default key is `V`
   (changeable in Settings).

**Two users on one computer:** log in as one user in a normal window and as another nickname in a **private /
incognito window**. The private window uses a separate certificate (identity). **Wear headphones** or mute one side
to avoid echo and feedback.

### 6.2 Administering with SuperUser (creating channels)

In the local environment Mumble's admin account **SuperUser** can also be used from the web:

1. On the login screen use nickname `SuperUser` and **Server password** `superuser` (`MUMBLE_SUPERUSER_PASSWORD`
   in `.env`).
2. **Right-click a channel → Create sub-channel…** → type a name and click **Create**. With "Temporary channel" the
   channel is deleted when everyone leaves.
3. Double-click a channel or use **right-click → Join channel** to enter it.
4. **Right-click users** for the desktop Mumble admin menu: kick, ban (certificate only), mute/deafen, priority
   speaker, move to your channel, register, reset comment, info, private message. Nickname, friend and "ignore
   messages" are stored only in your own browser.
5. **Right-click channels**: listen, add, edit, remove, link/unlink, copy URL, send message, hide/pin when
   filtering. The funnel button above the channel list hides empty channels. A copied URL
   (`https://mumweb.test/#kanal=/Oyun`) takes whoever logs in with it straight to that channel.
6. **Password-protected channel:** as SuperUser right-click a channel → **Edit…** → "Channel password". A normal
   user sees the channel with a red lock and is asked for the password on double-click ("Remember in this browser"
   skips the prompt next time; remove it under Settings → Channel passwords). In the desktop Mumble the same
   password is used by adding it under *Server → Access tokens*.

SuperUser **cannot speak** in Mumble (by design): use a normal nickname for audio tests and SuperUser (for example in
a private window) for administration. Normal (unregistered) users cannot create channels with Mumble's default
permissions; if they try they see "You don't have permission for this action." in the chat. This comes from the
server's ACL settings.

> Web SuperUser login is controlled by `MUMWEB_ALLOW_SUPERUSER`. It is **on** in the local environment and **off**
> in the production compose file; the web login page is open to anyone who knows the URL, so use the desktop client
> for administration on a real server.

### 6.3 With the desktop Mumble client

1. In Mumble choose **Server → Connect → Add new…**:
   - Address: `mumweb.test`
   - Port: `64738`
   - Username: any name
2. The Mumble server also uses the mkcert certificate from `certs\`, and mkcert's root is installed in Windows, so
   the client connects **without a certificate prompt**. (Connecting via `localhost` or an IP may prompt because the
   name does not match; use `mumweb.test`.)
3. You see the web users; speaking, chat and channel switching should work both ways.

For administration from the desktop connect as `SuperUser` / `superuser` and right-click a channel → **Add**.

For scenarios worth testing see the [manual acceptance tests](TESTING.md).

## 7. Settings (.env)

No `.env` file is needed with the defaults. To change something:

```powershell
copy .env.example .env
notepad .env
docker compose up -d       # applies the changes
```

| Variable | Default | Description |
|---|---|---|
| `MUMWEB_DOMAIN` | `mumweb.test` | The name in your hosts file and certificate |
| `HTTPS_PORT` | `443` | Port the browser connects to |
| `HTTP_PORT` | `80` | HTTP → HTTPS redirect |
| `MUMBLE_PORT` | `64738` | Port the desktop Mumble client connects to |
| `MUMBLE_SUPERUSER_PASSWORD` | `superuser` | Mumble SuperUser password (set on first run) |
| `MUMWEB_ALLOW_SUPERUSER` | `true` | Allow `SuperUser` login from the web |
| `MUMWEB_LOG_LEVEL` | `debug` | Gateway log level |

### Port conflicts (80/443 in use)

If IIS, Skype, VMware or another proxy uses 80/443, Caddy will not start ("port is already allocated"). To see which
program uses a port:

```powershell
netstat -ano | findstr ":443"
Get-Process -Id <PID>          # the number in the last column
```

If `netstat` shows nothing but the port is still unavailable, Windows (Hyper-V/WSL) may have reserved the range:

```powershell
netsh interface ipv4 show excludedportrange protocol=tcp
```

You can stop the program or choose other ports in `.env`:

```
HTTPS_PORT=8443
HTTP_PORT=8080
```

The address then becomes **<https://mumweb.test:8443>** (the origin the gateway allows is adjusted automatically).
Note that the redirect from the HTTP port does not know the port; use `https://…:8443` directly.

## 8. After changing the code

The UI and gateway are built into the image. After a change just rebuild mumweb:

```powershell
docker compose up -d --build mumweb
```

Reload the page. Because the app can be added to the home screen, a service worker caches static files. The page
itself is always loaded from the network, so a normal reload is enough. If you still see the old UI press
**Ctrl+F5**; if that fails, use *Application → Service workers → Unregister* in the developer tools.

If you change the UI often and have Node installed, `npm run dev` in `web` gives live reload (Vite). The gateway
must be running on `localhost:8080` for that (see the [README](../README.md#building-from-source), `make dev`).

## 9. Logs, stopping, resetting

```powershell
docker compose logs -f mumweb        # gateway (JSON): connections, errors
docker compose logs -f mumble        # Mumble server
docker compose logs -f caddy         # HTTPS / proxy

docker compose stop                  # stop (data stays)
docker compose start                 # start again
docker compose down                  # remove containers (Mumble channels/registrations stay in the volume)
docker compose down -v               # reset everything: channels, registered users, SuperUser password
```

## 10. Testing from another device on the same network

To try from a phone or another computer, that device must (a) resolve `mumweb.test` to your computer and (b) trust
the mkcert certificate.

1. **Your computer's local IP:** `ipconfig` → *IPv4 Address* (for example `192.168.1.20`).
2. **Generate the certificate for this IP too** (optional but recommended):
   ```powershell
   mkcert -cert-file certs\cert.pem -key-file certs\key.pem mumweb.test localhost 127.0.0.1 ::1 192.168.1.20
   docker compose up -d --force-recreate certs mumble caddy
   ```
3. **Firewall:** in an Administrator PowerShell:
   ```powershell
   New-NetFirewallRule -DisplayName "mumweb local" -Direction Inbound -Protocol TCP -LocalPort 80,443,64738 -Action Allow
   ```
4. **Install the root certificate on the other device:** the root certificate is `rootCA.pem` in the folder shown
   by `mkcert -CAROOT` (**never share `rootCA-key.pem`**).
   - Another Windows PC: copy it as `rootCA.crt` and double-click → *Install certificate* → *Local Machine* →
     *Trusted Root Certification Authorities*.
   - Android: copy the file to the phone → *Settings → Security → Encryption & credentials → Install a certificate
     → CA certificate*. (Menu names vary by manufacturer.)
5. **Name resolution:**
   - Another PC: add `192.168.1.20 mumweb.test` to its hosts file.
   - On Android the hosts file cannot be edited. If your network has a local DNS (for example AdGuard Home /
     Pi-hole "DNS rewrite", or a local DNS entry on some modems) add a `mumweb.test → 192.168.1.20` record.
6. Open `https://mumweb.test` on the other device.

> iOS / Safari is not supported yet (older Safari has no WebCodecs Opus).

## 11. Troubleshooting

| Symptom | Fix |
|---|---|
| `docker compose up` stops with "service certs didn't complete successfully" / the `caddy` log shows `open /certs/cert.pem: no such file` | The certificate was not generated or has a different name: run the command in [3.2](#32-generate-a-certificate-for-mumwebtest) inside `deploy\local`, then `docker compose up -d` |
| The desktop Mumble still asks about the certificate | Connect using the name `mumweb.test`; if you renewed the certificate later run `docker compose up -d --force-recreate certs mumble` |
| `NET::ERR_CERT_AUTHORITY_INVALID` / "Your connection is not private" | `mkcert -install` was not run, or was run while the browser was open: fully close and reopen the browser. For Firefox see the setting in [3.1](#31-install-the-local-certificate-authority-once) |
| `NET::ERR_CERT_COMMON_NAME_INVALID` | The certificate was generated for another name; regenerate it including the name you open (`mumweb.test`) |
| "This site can't be reached" / `DNS_PROBE_FINISHED_NXDOMAIN` | The hosts line is missing or wrong; check with `ping mumweb.test`, `ipconfig /flushdns`. Turn off any VPN or corporate proxy in the browser |
| `port is already allocated` | 80/443/64738 in use: see [Port conflicts](#port-conflicts-80443-in-use) |
| The page opens but always says "Reconnecting"; `docker compose logs mumweb` shows `ws origin rejected` | The address in the address bar does not match `MUMWEB_DOMAIN` / `HTTPS_PORT` in `.env`. Use `https://mumweb.test` instead of `localhost` |
| "Cannot reach the Mumble server" | Check that `mumble` is running with `docker compose ps`; `docker compose logs mumble` |
| "Microphone unavailable" / "Could not open the microphone" | The real error is in the parentheses under the warning. `NotReadableError` / `AbortError`: another app (Teams, Discord, OBS…) is using the microphone, or Windows blocks it → under *Settings → Privacy & security → Microphone* both "Microphone access" and "Let desktop apps access your microphone" must be on. `NotFoundError` ("Requested device not found", no permission prompt at all): the browser sees no microphone; the warning shows a device count of 0 → check that the device exists under *Settings → System → Sound → Input*, that both privacy switches are on, that microphone redirection is enabled in Remote Desktop / VMs, and fully restart the browser. After fixing it press **Try again** |
| No microphone prompt / "permission denied" | Allow the site from the microphone / lock icon in the address bar (Firefox: the lock → permissions next to *Connection secure*), then **Try again** |
| The other side hears their own voice as an echo | Their speaker sound is going into their microphone: use headphones or switch to push-to-talk |
| Choppy audio | Check that Docker Desktop has enough CPU/RAM (*Settings → Resources*); close heavy background work; test with two different devices if possible |
| Caddyfile parse error in the Caddy log, or a `\r` error while building | Files were checked out with CRLF (a clone from before `.gitattributes` was added). Easiest is to clone again. To fix in place: `git rm --cached -r . ; git reset --hard` (**uncommitted changes are lost**) |
| My changes do not show up | `docker compose up -d --build mumweb`, then Ctrl+F5 in the browser ([8](#8-after-changing-the-code)) |

## 12. macOS / Linux differences

The steps are the same. What changes:

- Installing mkcert: macOS `brew install mkcert nss` · Debian/Ubuntu `sudo apt install mkcert libnss3-tools`
- hosts file: `/etc/hosts`, for example
  `echo "127.0.0.1 mumweb.test" | sudo tee -a /etc/hosts`
- Use `/` instead of `\` in commands: `mkcert -cert-file certs/cert.pem -key-file certs/key.pem mumweb.test localhost 127.0.0.1 ::1`
- On Linux, ports below 1024 publish fine with Docker; just make sure no other web server uses them.

## Other environments

| File | When |
|---|---|
| `deploy/local/compose.yaml` | **This guide:** end-to-end trial over HTTPS, comparing with the desktop client |
| `deploy/docker-compose.dev.yml` (`make dev`) | Gateway development and Go integration tests; no HTTPS, `http://localhost:8080` (localhost counts as a secure context) |
| `deploy/docker-compose.yml` | Installing on a real server (Let's Encrypt), see the [README](../README.md#quick-start-production) |
