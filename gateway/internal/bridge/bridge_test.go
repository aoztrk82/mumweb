package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
)

func TestOriginAndSessionLimits(t *testing.T) {
	h := New(Config{MumbleAddr: "127.0.0.1:1", AllowedOrigins: []string{"https://ses.example.com"}, MaxSessions: 1})
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx := context.Background()

	_, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://evil.example"}}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bad origin: err=%v resp=%v", err, resp)
	}

	good := &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://ses.example.com"}}}
	c1, _, err := websocket.Dial(ctx, url, good)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.CloseNow()
	_, resp, err = websocket.Dial(ctx, url, good)
	if err == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("second session: err=%v resp=%v", err, resp)
	}
}

func TestIPRateLimit(t *testing.T) {
	l := newIPLimiter(2)
	if !l.allow("1.2.3.4") || !l.allow("1.2.3.4") || l.allow("1.2.3.4") {
		t.Fatal("limit not enforced")
	}
	if !l.allow("5.6.7.8") {
		t.Fatal("limit shared between IPs")
	}
}

// --- integration test against a real server (e.g. the dev compose stack) ---

// mumbleAddr returns the test server address or skips.
// Run with: MUMWEB_TEST_MUMBLE_ADDR=localhost:64739 go test ./internal/bridge
func mumbleAddr(t *testing.T) string {
	addr := os.Getenv("MUMWEB_TEST_MUMBLE_ADDR")
	if addr == "" {
		t.Skip("MUMWEB_TEST_MUMBLE_ADDR not set")
	}
	return addr
}

type wsClient struct {
	t  *testing.T
	ws *websocket.Conn
}

func dialBridge(t *testing.T, url string, hello map[string]any) *wsClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	ws.SetReadLimit(1 << 20)
	hello["t"] = "hello"
	data, _ := json.Marshal(hello)
	if err := ws.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatal(err)
	}
	return &wsClient{t: t, ws: ws}
}

// next returns the next message: a decoded JSON object, or nil and the raw binary frame.
func (c *wsClient) next() (map[string]any, []byte) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	typ, data, err := c.ws.Read(ctx)
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	if typ == websocket.MessageBinary {
		return nil, data
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		c.t.Fatal(err)
	}
	return m, nil
}

// until reads messages until one with the given t arrives.
func (c *wsClient) until(kind string) map[string]any {
	c.t.Helper()
	for {
		if m, _ := c.next(); m != nil && m["t"] == kind {
			return m
		}
	}
}

func (c *wsClient) send(v map[string]any) {
	c.t.Helper()
	data, _ := json.Marshal(v)
	if err := c.ws.Write(context.Background(), websocket.MessageText, data); err != nil {
		c.t.Fatal(err)
	}
}

// waitEvent reads observer events until match returns true.
func waitEvent[T mumble.Event](t *testing.T, mc *mumble.Client, match func(T) bool) T {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-mc.Events():
			if !ok {
				t.Fatalf("observer disconnected: %v", mc.Err())
			}
			if e, ok := ev.(T); ok && match(e) {
				return e
			}
		case <-timeout:
			var zero T
			t.Fatalf("timed out waiting for %T", zero)
		}
	}
}

func TestBridgeEndToEnd(t *testing.T) {
	addr := mumbleAddr(t)
	baseline := runtime.NumGoroutine()
	h := New(Config{MumbleAddr: addr})
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	obs, err := mumble.Dial(context.Background(), mumble.Config{Addr: addr, Username: "observer"})
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Close()

	// First login without certificate: gateway generates one.
	c := dialBridge(t, url, map[string]any{"username": "wsuser", "password": "", "cert": nil})
	cert := c.until("cert")
	if !strings.Contains(cert["pem"].(string), "BEGIN CERTIFICATE") || !strings.Contains(cert["key"].(string), "PRIVATE KEY") {
		t.Fatalf("bad cert message")
	}
	var sawRoot, sawObserver bool
	var ready map[string]any
	for ready == nil {
		m, _ := c.next()
		switch m["t"] {
		case "channel":
			sawRoot = sawRoot || (m["id"] == 0.0 && m["parent"] == nil)
		case "user":
			sawObserver = sawObserver || m["name"] == "observer"
		case "ready":
			ready = m
		}
	}
	if !sawRoot || !sawObserver {
		t.Fatalf("initial state incomplete: root=%v observer=%v", sawRoot, sawObserver)
	}
	if ready["messageLength"] == 0.0 {
		t.Fatalf("ready lacks ServerConfig data: %v", ready)
	}
	mySession := uint32(ready["session"].(float64))
	joined := waitEvent(t, obs, func(e mumble.UserEvent) bool { return e.User.Name == "wsuser" })
	firstHash := joined.User.Hash

	// ping -> pong
	c.send(map[string]any{"t": "ping", "ts": 123})
	if m := c.until("pong"); m["ts"] != 123.0 {
		t.Fatalf("pong %v", m)
	}

	// browser -> Mumble text
	c.send(map[string]any{"t": "text", "message": "selam"})
	waitEvent(t, obs, func(e mumble.TextEvent) bool { return e.Actor == mySession && e.Message == "selam" })

	// Mumble -> browser text
	if err := obs.SendText("merhaba", []uint32{0}, nil); err != nil {
		t.Fatal(err)
	}
	if m := c.until("text"); m["message"] != "merhaba" || m["from"] != float64(obs.Session()) {
		t.Fatalf("text %v", m)
	}

	// setSelf is reflected to others
	c.send(map[string]any{"t": "setSelf", "mute": true, "deaf": false})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return e.User.Session == mySession && e.User.SelfMute })
	c.send(map[string]any{"t": "setSelf", "mute": false})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return e.User.Session == mySession && !e.User.SelfMute })

	// browser -> Mumble audio
	frame := []byte{0x01, 0x01, 5, 0, 0, 0, 0, 0, 0, 0, 0xf8, 0xff, 0xfe}
	if err := c.ws.Write(context.Background(), websocket.MessageBinary, frame); err != nil {
		t.Fatal(err)
	}
	a := waitEvent(t, obs, func(e mumble.AudioEvent) bool { return e.Packet.Sender == mySession })
	if a.Packet.FrameNumber != 5 || !a.Packet.Terminator || len(a.Packet.Opus) != 3 {
		t.Fatalf("observer got %+v", a.Packet)
	}

	// Mumble -> browser audio
	if err := obs.SendAudio(mumble.AudioPacket{FrameNumber: 9, Opus: []byte{0xf8, 0xff, 0xfe}, Terminator: true}); err != nil {
		t.Fatal(err)
	}
	for {
		_, bin := c.next()
		if bin == nil {
			continue
		}
		p, err := parseIncomingForTest(bin)
		if err != nil || p.Sender != obs.Session() || p.FrameNumber != 9 || !p.Terminator {
			t.Fatalf("browser got %+v err=%v", p, err)
		}
		break
	}

	// Closing the WebSocket drops the Mumble user.
	c.ws.Close(websocket.StatusNormalClosure, "")
	waitEvent(t, obs, func(e mumble.UserRemoveEvent) bool { return e.Session == mySession })

	// Second login with the stored certificate keeps the identity.
	c2 := dialBridge(t, url, map[string]any{"username": "wsuser", "cert": map[string]any{"pem": cert["pem"], "key": cert["key"]}})
	defer c2.ws.CloseNow()
	if m := c2.until("ready"); m == nil {
		t.Fatal("no ready")
	}
	again := waitEvent(t, obs, func(e mumble.UserEvent) bool { return e.User.Name == "wsuser" })
	if firstHash == "" || again.User.Hash != firstHash {
		t.Fatalf("certificate hash changed: %q -> %q", firstHash, again.User.Hash)
	}

	// No goroutines left behind once all sessions are gone.
	c2.ws.Close(websocket.StatusNormalClosure, "")
	waitEvent(t, obs, func(e mumble.UserRemoveEvent) bool { return e.Session != obs.Session() })
	obs.Close()
	srv.Close()
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > baseline || h.Sessions() != 0 {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			t.Fatalf("goroutine leak: %d > %d (sessions=%d)\n%s", runtime.NumGoroutine(), baseline, h.Sessions(), buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestBridgeReject(t *testing.T) {
	addr := mumbleAddr(t)
	srv := httptest.NewServer(New(Config{MumbleAddr: addr}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	c := dialBridge(t, url, map[string]any{"username": "SuperUser", "password": "definitely-wrong"})
	m := c.until("reject")
	if m["reason"] != "WrongUserPW" {
		t.Fatalf("reject %v", m)
	}
	_, _, err := c.ws.Read(context.Background())
	if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatalf("close: %v", err)
	}
}

func TestBridgeProtocolViolation(t *testing.T) {
	srv := httptest.NewServer(New(Config{MumbleAddr: "127.0.0.1:1"}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ws, _, err := websocket.Dial(context.Background(), url, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = ws.Write(context.Background(), websocket.MessageText, []byte(`{"t":"joinChannel","channelId":1}`))
	_, _, err = ws.Read(context.Background())
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("want policy violation, got %v", err)
	}
	var ce websocket.CloseError
	if !errors.As(err, &ce) {
		t.Fatal(err)
	}
}
