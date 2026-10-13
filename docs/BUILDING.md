# Building, testing and publishing

**English** | [Türkçe](tr/BUILDING.md)

You only need this page to build mumweb yourself, change the code, run the tests or publish a release. To just run
mumweb, use the ready-made Docker image from the [README](../README.md#quick-start-production).

## Build and run from source

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
another machine ([DEVELOPMENT.md](DEVELOPMENT.md)).

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
- **Manual acceptance checklist:** [TESTING.md](TESTING.md).

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

## Build the Docker image locally

```sh
make docker                       # builds mumweb:dev for your machine
```

To run the production stack with your own build instead of the Docker Hub image, add the override file:

```sh
cd deploy
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## Publishing a release (maintainers)

Images go to Docker Hub as [`aoztrk82/mumweb`](https://hub.docker.com/r/aoztrk82/mumweb) for `linux/amd64` and
`linux/arm64`. The Dockerfile cross-compiles Go on the build machine, so no CPU emulation is needed for the build.

```sh
docker login                                                                   # once
docker buildx create --name mumweb-builder --driver docker-container --bootstrap   # once
make publish VERSION=0.1.0       # pushes :latest and :0.1.0
```

Afterwards check the result: `docker buildx imagetools inspect aoztrk82/mumweb:latest` should list both platforms,
and a fresh `docker run -p 8080:8080 aoztrk82/mumweb:0.1.0` should answer `curl localhost:8080/healthz` with `ok`.
Bump `release` in `gateway/cmd/mumweb/main.go` and the default `VERSION` in the `Makefile` together, and record the
release in [PROGRESS.md](PROGRESS.md).
