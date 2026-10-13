# syntax=docker/dockerfile:1

# --- web: build the Svelte SPA into gateway/web/dist ---
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- gateway: build the Go binary with the SPA embedded ---
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS gateway
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src/gateway
COPY gateway/go.mod gateway/go.sum ./
RUN go mod download
COPY gateway/ ./
COPY --from=web /src/gateway/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/mumweb ./cmd/mumweb

# --- runtime ---
FROM alpine:3.24
RUN addgroup -S mumweb && adduser -S -G mumweb -H mumweb
COPY --from=gateway /out/mumweb /usr/local/bin/mumweb
USER mumweb
ENV MUMWEB_LISTEN=:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/mumweb"]
