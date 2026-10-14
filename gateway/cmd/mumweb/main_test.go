package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/aoztrk82/mumweb/gateway/internal/bridge"
)

func TestSecurityHeaders(t *testing.T) {
	dist := fstest.MapFS{"index.html": {Data: []byte("<!doctype html>")}, "admin/index.html": {Data: []byte("<!doctype html>")}}
	srv := httptest.NewServer(securityHeaders(newMux(dist, bridge.New(bridge.Config{MumbleAddr: "127.0.0.1:1"}))))
	defer srv.Close()

	for _, path := range []string{"/", "/admin/", "/config", "/healthz", "/missing"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		h := res.Header
		csp := h.Get("Content-Security-Policy")
		for _, want := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'", "object-src 'none'", "base-uri 'none'"} {
			if !strings.Contains(csp, want) {
				t.Errorf("%s: CSP lacks %q: %s", path, want, csp)
			}
		}
		if strings.Contains(csp, "unsafe-eval") || strings.Contains(strings.SplitN(strings.SplitAfter(csp, "script-src")[1], ";", 2)[0], "unsafe-inline") {
			t.Errorf("%s: scripts must not allow unsafe-eval/unsafe-inline: %s", path, csp)
		}
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("X-Frame-Options") != "DENY" || h.Get("Referrer-Policy") != "no-referrer" || !strings.Contains(h.Get("Permissions-Policy"), "microphone=(self)") {
			t.Errorf("%s: missing hardening headers: %v", path, h)
		}
	}
}
