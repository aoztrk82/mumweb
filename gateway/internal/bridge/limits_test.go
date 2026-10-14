package bridge

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestPerIPSessionLimit(t *testing.T) {
	h := New(Config{MumbleAddr: "127.0.0.1:1", MaxPerIP: 2})
	if !h.acquireIP("a") || !h.acquireIP("a") || h.acquireIP("a") {
		t.Fatal("limit of 2 not enforced")
	}
	if !h.acquireIP("b") {
		t.Fatal("limit leaks to another IP")
	}
	h.releaseIP("a")
	if !h.acquireIP("a") {
		t.Fatal("release did not free a slot")
	}
	h.releaseIP("a")
	h.releaseIP("a")
	h.releaseIP("b")
	if len(h.ipSessions) != 0 {
		t.Fatalf("leaked entries: %v", h.ipSessions)
	}
	off := New(Config{MumbleAddr: "127.0.0.1:1"})
	for i := 0; i < 100; i++ {
		if !off.acquireIP("a") {
			t.Fatal("limit 0 must be off")
		}
	}
}

// Open WebSockets that never send hello hold a session slot until the hello timeout.
func TestPerIPSessionLimitOverHTTP(t *testing.T) {
	srv := httptest.NewServer(New(Config{MumbleAddr: "127.0.0.1:1", MaxPerIP: 2}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		c, _, err := websocket.Dial(ctx, url, nil)
		if err != nil {
			t.Fatalf("connection %d: %v", i, err)
		}
		defer c.CloseNow()
	}
	if _, resp, err := websocket.Dial(ctx, url, nil); err == nil || resp == nil || resp.StatusCode != 429 {
		t.Fatalf("third connection from one IP: err=%v resp=%v", err, resp)
	}
}
