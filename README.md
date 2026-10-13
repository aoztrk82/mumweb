# mumweb

Browser client for [Mumble](https://www.mumble.info/) voice servers.

```
Svelte SPA  <--WebSocket-->  Go gateway  <--TLS/TCP-->  mumble-server
```

The Go gateway speaks the native Mumble protocol to the server and serves the
web UI itself (embedded via `go:embed`). The UI language is Turkish.

> Status: early scaffold (phase 0). No voice/chat functionality yet.
> See [docs/PROGRESS.md](docs/PROGRESS.md).

## Layout

| Path | Purpose |
|---|---|
| `gateway/cmd/mumweb` | HTTP/WebSocket gateway (serves the SPA, `/healthz`) |
| `gateway/cmd/mumprobe` | CLI for probing a Mumble server directly |
| `gateway/internal/` | `mumble` (protocol client), `bridge` (WS ↔ Mumble), `certs` |
| `gateway/proto/` | Upstream `Mumble.proto` / `MumbleUDP.proto` + generated Go code |
| `gateway/web/dist` | Vite build output, embedded into the binary |
| `web/` | Svelte 5 + TypeScript SPA (Vite) |
| `deploy/` | Docker Compose files and env template |

## Requirements

Go 1.27+, Node 24 LTS, `protoc` + `protoc-gen-go` (only for `make proto`),
Docker with Compose, GNU Make.

## Usage

```sh
make gateway        # build SPA, then bin/mumweb and bin/mumprobe
./bin/mumweb        # listens on $MUMWEB_LISTEN (default :8080)

cp deploy/.env.example deploy/.env   # set MUMBLE_SUPERUSER_PASSWORD
make dev            # mumble-server + gateway via docker compose
make test           # go test + vitest
make docker         # production image (mumweb:dev)
make proto          # regenerate Go code from gateway/proto
```

For frontend work run `npm run dev` in `web/`; it proxies `/ws` and `/healthz`
to the gateway on `localhost:8080`.

## License

MIT
