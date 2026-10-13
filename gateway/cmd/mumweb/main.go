// Command mumweb is the HTTP/WebSocket gateway between the browser client and a Mumble server.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aoztrk82/mumweb/gateway/internal/bridge"
	"github.com/aoztrk82/mumweb/gateway/web"
)

const release = "mumweb 0.1"

const noBuildPage = `<!doctype html>
<html lang="tr"><head><meta charset="utf-8"><title>mumweb</title></head>
<body><h1>mumweb</h1><p>Web arayüzü derlenmemiş (build yok). <code>make web</code> çalıştırıp gateway'i yeniden derleyin.</p></body></html>
`

func main() {
	var level slog.Level
	if err := level.UnmarshalText([]byte(env("MUMWEB_LOG_LEVEL", "info"))); err != nil {
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	addr := env("MUMWEB_LISTEN", ":8080")
	bcfg := bridge.Config{
		MumbleAddr:     env("MUMWEB_MUMBLE_ADDR", "mumble:64738"),
		TLSVerify:      envBool("MUMWEB_TLS_VERIFY", false),
		Fingerprint:    env("MUMWEB_TLS_FINGERPRINT", ""),
		MaxSessions:    envInt("MUMWEB_MAX_SESSIONS", 50),
		AllowedOrigins: envList("MUMWEB_ALLOWED_ORIGINS"),
		ConnsPerMinute: envInt("MUMWEB_CONN_RATE", 10),
		TrustProxy:     envBool("MUMWEB_TRUST_PROXY", false),
		Release:        release,
		Logger:         logger,
	}
	ws := bridge.New(bcfg)

	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(web.Dist(), ws),
		ReadHeaderTimeout: 10 * time.Second,
	}
	srv.RegisterOnShutdown(ws.Shutdown)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("listening", "addr", addr, "mumble", bcfg.MumbleAddr, "tlsVerify", bcfg.TLSVerify,
			"pinned", bcfg.Fingerprint != "", "maxSessions", bcfg.MaxSessions, "origins", bcfg.AllowedOrigins)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "err", err)
	}
}

func newMux(dist fs.FS, ws http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /ws", ws)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	if _, err := fs.Stat(dist, "index.html"); err != nil {
		slog.Warn("web/dist has no index.html; serving placeholder page")
		mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(noBuildPage))
		})
	} else {
		mux.Handle("GET /", cacheHeaders(http.FileServerFS(dist)))
	}
	return mux
}

func init() {
	// Not in Go's built-in table; browsers want this type for the PWA manifest.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// cacheHeaders lets browsers keep hashed build assets forever and revalidate
// everything else (index.html, sw.js, manifest), so updates show up promptly.
func cacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, err := strconv.ParseBool(env(key, strconv.FormatBool(def)))
	if err != nil {
		slog.Warn("invalid boolean, using default", "var", key, "default", def)
		return def
	}
	return v
}

func envInt(key string, def int) int {
	v, err := strconv.Atoi(env(key, strconv.Itoa(def)))
	if err != nil || v < 0 {
		slog.Warn("invalid integer, using default", "var", key, "default", def)
		return def
	}
	return v
}

// envList parses a comma-separated list, ignoring empty entries.
func envList(key string) []string {
	var out []string
	for _, p := range strings.Split(os.Getenv(key), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
