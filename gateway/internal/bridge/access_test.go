package bridge

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestPasswordMatches(t *testing.T) {
	if !passwordMatches("s3cret", "s3cret") || passwordMatches("s3cret", "s3cre") || passwordMatches("", "x") {
		t.Fatal("passwordMatches wrong")
	}
}

func TestFailTracker(t *testing.T) {
	f := newFailTracker()
	now := time.Now()
	for i := 0; i < accessMaxFailures; i++ {
		if f.locked("1.1.1.1", now) {
			t.Fatalf("locked after only %d failures", i)
		}
		f.fail("1.1.1.1", now)
	}
	if !f.locked("1.1.1.1", now) {
		t.Fatal("not locked after max failures")
	}
	if f.locked("2.2.2.2", now) {
		t.Fatal("lock leaks to another IP")
	}
	if f.locked("1.1.1.1", now.Add(accessWindow+time.Second)) {
		t.Fatal("lock does not expire")
	}
	f.ok("1.1.1.1")
	if f.locked("1.1.1.1", now) {
		t.Fatal("ok() did not clear the record")
	}
}

// helloReply sends a hello and returns the first JSON message the gateway answers with.
func helloReply(t *testing.T, url string, hello map[string]any) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	hello["t"] = "hello"
	data, _ := json.Marshal(hello)
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var m map[string]any
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		// Past the access check the gateway tries Mumble (unreachable here).
		if m["t"] == "reject" || m["t"] == "error" {
			return m
		}
	}
}

func TestAccessPassword(t *testing.T) {
	accessFailDelay = time.Millisecond
	t.Cleanup(func() { accessFailDelay = time.Second })

	h := New(Config{MumbleAddr: "127.0.0.1:1", AccessPassword: "open-sesame"})
	if !h.AccessPasswordRequired() {
		t.Fatal("AccessPasswordRequired should be true")
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	if m := helloReply(t, url, map[string]any{"username": "a"}); m["reason"] != rejectAccessRequired {
		t.Fatalf("missing password: %v", m)
	}
	if m := helloReply(t, url, map[string]any{"username": "a", "accessPassword": "nope"}); m["reason"] != rejectAccessWrong {
		t.Fatalf("wrong password: %v", m)
	}
	// The Mumble server password must not satisfy the web access password.
	if m := helloReply(t, url, map[string]any{"username": "a", "password": "open-sesame"}); m["reason"] != rejectAccessRequired {
		t.Fatalf("server password accepted as access password: %v", m)
	}
	// The right password gets past the check (and then fails to reach Mumble).
	if m := helloReply(t, url, map[string]any{"username": "a", "accessPassword": "open-sesame"}); m["t"] != "error" || m["code"] != errCodeMumbleUnavailable {
		t.Fatalf("right password: %v", m)
	}

	// Enough wrong guesses lock the IP out, even for the right password.
	for i := 0; i < accessMaxFailures; i++ {
		helloReply(t, url, map[string]any{"username": "a", "accessPassword": "bad"})
	}
	if m := helloReply(t, url, map[string]any{"username": "a", "accessPassword": "open-sesame"}); m["reason"] != rejectAccessRateLimited {
		t.Fatalf("lockout: %v", m)
	}
}

func TestNoAccessPassword(t *testing.T) {
	h := New(Config{MumbleAddr: "127.0.0.1:1"})
	if h.AccessPasswordRequired() {
		t.Fatal("AccessPasswordRequired should be false")
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	if m := helloReply(t, url, map[string]any{"username": "a"}); m["t"] != "error" {
		t.Fatalf("open gateway: %v", m)
	}
}
