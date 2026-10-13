package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/aoztrk82/mumweb/gateway/internal/certs"
	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
)

func TestOriginAndSessionLimits(t *testing.T) {
	h := New(Config{MumbleAddr: "127.0.0.1:1", AllowedOrigins: []string{"https://ses.example.com:443"}, MaxSessions: 1})
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
	srv := httptest.NewServer(New(Config{MumbleAddr: addr, AllowSuperUser: true}))
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

func TestSuperUserDisabled(t *testing.T) {
	// Refused before any Mumble connection is made.
	srv := httptest.NewServer(New(Config{MumbleAddr: "127.0.0.1:1"}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	for _, name := range []string{"SuperUser", "superuser", " SUPERUSER "} {
		c := dialBridge(t, url, map[string]any{"username": name, "password": "x"})
		if m := c.until("reject"); m["reason"] != "SuperUserDisabled" {
			t.Fatalf("%q: %v", name, m)
		}
		c.ws.CloseNow()
	}
}

func TestCreateChannel(t *testing.T) {
	addr := mumbleAddr(t)
	srv := httptest.NewServer(New(Config{MumbleAddr: addr, AllowSuperUser: true}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	// An ordinary user may not create permanent channels (default ACL).
	c := dialBridge(t, url, map[string]any{"username": "kanalci"})
	c.until("ready")
	c.send(map[string]any{"t": "createChannel", "parent": 0, "name": "izinsiz", "temporary": false})
	if m := c.until("denied"); m["type"] != "Permission" {
		t.Fatalf("denied %v", m)
	}
	c.ws.CloseNow()

	pw := os.Getenv("MUMWEB_TEST_SUPERUSER_PASSWORD")
	if pw == "" {
		t.Skip("MUMWEB_TEST_SUPERUSER_PASSWORD not set")
	}
	su := dialBridge(t, url, map[string]any{"username": "SuperUser", "password": pw})
	defer su.ws.CloseNow()
	su.until("ready")
	name := "test-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	su.send(map[string]any{"t": "createChannel", "parent": 0, "name": name, "temporary": true})
	for {
		m := su.until("channel")
		if m["name"] == name {
			if m["temporary"] != true || m["parent"] != 0.0 {
				t.Fatalf("channel %v", m)
			}
			break
		}
	}
}

// TestAdminActions exercises the user context-menu actions as SuperUser.
func TestAdminActions(t *testing.T) {
	addr := mumbleAddr(t)
	pw := os.Getenv("MUMWEB_TEST_SUPERUSER_PASSWORD")
	if pw == "" {
		t.Skip("MUMWEB_TEST_SUPERUSER_PASSWORD not set")
	}
	srv := httptest.NewServer(New(Config{MumbleAddr: addr, AllowSuperUser: true}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	suffix := strconv.FormatInt(time.Now().UnixNano()%1e6, 36)

	obs, err := mumble.Dial(context.Background(), mumble.Config{Addr: addr, Username: "gozcu-" + suffix})
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Close()

	su := dialBridge(t, url, map[string]any{"username": "SuperUser", "password": pw})
	defer su.ws.CloseNow()
	ready := su.until("ready")
	if perms := uint32(ready["permissions"].(float64)); perms&(mumble.PermKick|mumble.PermBan|mumble.PermWrite) == 0 {
		t.Fatalf("root permissions %#x", perms)
	}
	victim := dialBridge(t, url, map[string]any{"username": "kurban-" + suffix})
	defer victim.ws.CloseNow()
	vs := uint32(victim.until("ready")["session"].(float64))

	// The server never sends PermissionQuery answers to SuperUser (it has every
	// permission), so ask as the ordinary user.
	victim.send(map[string]any{"t": "queryPermissions", "channelId": 0})
	if m := victim.until("permissions"); m["channelId"] != 0.0 || uint32(m["permissions"].(float64))&mumble.PermEnter == 0 {
		t.Fatalf("permissions %v", m)
	}
	isVictim := func(e mumble.UserEvent) bool { return e.User.Session == vs }

	su.send(map[string]any{"t": "setUser", "session": vs, "mute": true})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return isVictim(e) && e.User.Mute })
	su.send(map[string]any{"t": "setUser", "session": vs, "deaf": true})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return isVictim(e) && e.User.Deaf })
	su.send(map[string]any{"t": "setUser", "session": vs, "mute": false, "deaf": false})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return isVictim(e) && !e.User.Mute && !e.User.Deaf })
	su.send(map[string]any{"t": "setUser", "session": vs, "prioritySpeaker": true})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return isVictim(e) && e.User.PrioritySpeaker })

	// Move to another channel (any non-root channel), then back.
	var target uint32
	for _, c := range obs.State().Channels() {
		if c.ID != 0 {
			target = c.ID
			break
		}
	}
	if target != 0 {
		su.send(map[string]any{"t": "moveUser", "session": vs, "channelId": target})
		waitEvent(t, obs, func(e mumble.UserEvent) bool { return isVictim(e) && e.User.ChannelID == target })
		su.send(map[string]any{"t": "moveUser", "session": vs, "channelId": 0})
		waitEvent(t, obs, func(e mumble.UserEvent) bool { return isVictim(e) && e.User.ChannelID == 0 })
	}

	// Private message reaches only the victim.
	su.send(map[string]any{"t": "text", "session": vs, "message": "gizli"})
	if m := victim.until("text"); m["message"] != "gizli" || m["private"] != true {
		t.Fatalf("private text %v", m)
	}

	// User information.
	su.send(map[string]any{"t": "userStats", "session": vs})
	st := su.until("userStats")
	if st["session"] != float64(vs) || st["os"] != "Web" || st["version"] != "1.5.0" {
		t.Fatalf("userStats %v", st)
	}

	// Register the victim.
	su.send(map[string]any{"t": "register", "session": vs})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return isVictim(e) && e.User.Registered })

	// Kick.
	su.send(map[string]any{"t": "kick", "session": vs, "reason": "test"})
	if m := victim.until("userRemove"); m["session"] != float64(vs) || m["reason"] != "test" {
		t.Fatalf("kick %v", m)
	}

	// Ban by certificate only: the banned identity cannot return, others (same gateway IP) can.
	v2 := dialBridge(t, url, map[string]any{"username": "yasakli-" + suffix, "cert": nil})
	cert := v2.until("cert")
	v2s := uint32(v2.until("ready")["session"].(float64))
	su.send(map[string]any{"t": "ban", "session": v2s, "reason": "yasak"})
	if m := v2.until("userRemove"); m["session"] != float64(v2s) {
		t.Fatalf("ban kick %v", m)
	}
	v2.ws.CloseNow()

	again := dialBridge(t, url, map[string]any{"username": "yasakli2-" + suffix, "cert": map[string]any{"pem": cert["pem"], "key": cert["key"]}})
	for {
		m, _ := again.next()
		if m["t"] == "ready" {
			t.Fatal("banned certificate was let back in")
		}
		if m["t"] == "error" || m["t"] == "reject" {
			break
		}
	}
	again.ws.CloseNow()

	other := dialBridge(t, url, map[string]any{"username": "masum-" + suffix})
	defer other.ws.CloseNow()
	if m := other.until("ready"); m == nil {
		t.Fatal("other web users must still get in")
	}
}

// TestChannelActions exercises the channel context-menu actions as SuperUser.
func TestChannelActions(t *testing.T) {
	addr := mumbleAddr(t)
	pw := os.Getenv("MUMWEB_TEST_SUPERUSER_PASSWORD")
	if pw == "" {
		t.Skip("MUMWEB_TEST_SUPERUSER_PASSWORD not set")
	}
	srv := httptest.NewServer(New(Config{MumbleAddr: addr, AllowSuperUser: true}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	name := "kanal-" + strconv.FormatInt(time.Now().UnixNano()%1e6, 36)

	obs, err := mumble.Dial(context.Background(), mumble.Config{Addr: addr, Username: "gozcu-" + name})
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Close()

	su := dialBridge(t, url, map[string]any{"username": "SuperUser", "password": pw})
	defer su.ws.CloseNow()
	su.until("ready")

	su.send(map[string]any{"t": "createChannel", "parent": 0, "name": name})
	created := waitEvent(t, obs, func(e mumble.ChannelEvent) bool { return e.Channel.Name == name })
	id := created.Channel.ID
	isOurs := func(e mumble.ChannelEvent) bool { return e.Channel.ID == id }

	// Edit: rename, description, position, max users.
	su.send(map[string]any{"t": "editChannel", "channelId": id, "newName": name + "-2", "description": "<b>açıklama</b>", "position": 7, "maxUsers": 5})
	waitEvent(t, obs, func(e mumble.ChannelEvent) bool {
		c := e.Channel
		return isOurs(e) && c.Name == name+"-2" && c.Description == "<b>açıklama</b>" && c.Position == 7 && c.MaxUsers == 5
	})
	su.send(map[string]any{"t": "requestDescription", "channelId": id})

	// Link our channel (root) with it, then unlink. The browser must see both sides
	// change, in order (links are symmetric but announced on one side only).
	rootLinks := func() []any {
		for {
			m := su.until("channel")
			if m["id"] == 0.0 {
				lk, _ := m["links"].([]any)
				return lk
			}
		}
	}
	su.send(map[string]any{"t": "linkChannels", "channelId": 0, "add": []uint32{id}})
	waitEvent(t, obs, func(e mumble.ChannelEvent) bool { return isOurs(e) && slices.Contains(e.Channel.Links, 0) })
	for !slices.Contains(rootLinks(), any(float64(id))) {
	}
	su.send(map[string]any{"t": "linkChannels", "channelId": 0, "remove": []uint32{id}})
	waitEvent(t, obs, func(e mumble.ChannelEvent) bool { return isOurs(e) && !slices.Contains(e.Channel.Links, 0) })
	if lk := rootLinks(); slices.Contains(lk, any(float64(id))) {
		t.Fatalf("browser still sees root linked: %v", lk)
	}

	// A normal web user listens to the channel; the listening list is forwarded.
	l := dialBridge(t, url, map[string]any{"username": "dinleyici-" + name})
	defer l.ws.CloseNow()
	ls := uint32(l.until("ready")["session"].(float64))
	l.send(map[string]any{"t": "listen", "channelId": id, "on": true})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return e.User.Session == ls && slices.Contains(e.User.Listening, id) })
	for {
		m := su.until("user")
		if m["session"] == float64(ls) {
			if lst, _ := m["listening"].([]any); len(lst) == 1 && lst[0] == float64(id) {
				break
			}
		}
	}
	l.send(map[string]any{"t": "listen", "channelId": id, "on": false})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return e.User.Session == ls && len(e.User.Listening) == 0 })

	// Tree message to root reaches users in sub-channels.
	su.send(map[string]any{"t": "text", "channelId": 0, "tree": true, "message": "herkese"})
	if m := l.until("text"); m["message"] != "herkese" {
		t.Fatalf("tree text %v", m)
	}

	// Remove.
	su.send(map[string]any{"t": "removeChannel", "channelId": id})
	waitEvent(t, obs, func(e mumble.ChannelRemoveEvent) bool { return e.ID == id })
}

// TestChannelPassword covers password-protected channels (ACL "#password" tokens).
func TestChannelPassword(t *testing.T) {
	addr := mumbleAddr(t)
	pw := os.Getenv("MUMWEB_TEST_SUPERUSER_PASSWORD")
	if pw == "" {
		t.Skip("MUMWEB_TEST_SUPERUSER_PASSWORD not set")
	}
	srv := httptest.NewServer(New(Config{MumbleAddr: addr, AllowSuperUser: true}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	name := "sifreli-" + strconv.FormatInt(time.Now().UnixNano()%1e6, 36)

	obs, err := mumble.Dial(context.Background(), mumble.Config{Addr: addr, Username: "gozcu-" + name})
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Close()

	su := dialBridge(t, url, map[string]any{"username": "SuperUser", "password": pw})
	defer su.ws.CloseNow()
	su.until("ready")
	su.send(map[string]any{"t": "createChannel", "parent": 0, "name": name})
	id := waitEvent(t, obs, func(e mumble.ChannelEvent) bool { return e.Channel.Name == name }).Channel.ID
	defer func() { su.send(map[string]any{"t": "removeChannel", "channelId": id}) }()

	su.send(map[string]any{"t": "setChannelPassword", "channelId": id, "channelPassword": "gizli123"})
	if m := su.until("channelPassword"); m["password"] != "gizli123" {
		t.Fatalf("set %v", m)
	}
	su.send(map[string]any{"t": "queryChannelPassword", "channelId": id})
	if m := su.until("channelPassword"); m["password"] != "gizli123" || m["channelId"] != float64(id) {
		t.Fatalf("query %v", m)
	}

	// An ordinary user sees the channel as restricted and cannot enter.
	u := dialBridge(t, url, map[string]any{"username": "misafir-" + name})
	defer u.ws.CloseNow()
	locked := func(m map[string]any) bool {
		return m["id"] == float64(id) && m["restricted"] == true && m["canEnter"] == false
	}
	for {
		if m, _ := u.next(); m["t"] == "channel" && locked(m) {
			break
		}
	}
	u.until("ready")
	u.send(map[string]any{"t": "joinChannel", "channelId": id})
	if m := u.until("denied"); m["permission"] != float64(mumble.PermEnter) || m["channelId"] != float64(id) {
		t.Fatalf("denied %v", m)
	}

	// Wrong password: still denied.
	u.send(map[string]any{"t": "setTokens", "tokens": []string{"yanlis"}})
	u.send(map[string]any{"t": "joinChannel", "channelId": id})
	u.until("denied")

	// Right password: can_enter flips and the join works.
	u.send(map[string]any{"t": "setTokens", "tokens": []string{"yanlis", "gizli123"}})
	for {
		m := u.until("channel")
		if m["id"] == float64(id) && m["canEnter"] == true {
			break
		}
	}
	us := uint32(0)
	for _, x := range obs.State().Users() {
		if x.Name == "misafir-"+name {
			us = x.Session
		}
	}
	u.send(map[string]any{"t": "joinChannel", "channelId": id})
	waitEvent(t, obs, func(e mumble.UserEvent) bool { return e.User.Session == us && e.User.ChannelID == id })

	// Tokens sent at login apply from the start.
	v := dialBridge(t, url, map[string]any{"username": "anahtarli-" + name, "tokens": []string{"gizli123"}})
	defer v.ws.CloseNow()
	for {
		m, _ := v.next()
		if m["t"] == "channel" && m["id"] == float64(id) {
			if m["canEnter"] != true {
				t.Fatalf("login tokens not applied: %v", m)
			}
			break
		}
	}

	// Removing the password lifts the restriction. Wait first: the server rate-limits
	// ACL/ChannelState/TextMessage/own UserState per user (1/s, burst 5) and silently
	// drops the excess.
	time.Sleep(3 * time.Second)
	su.send(map[string]any{"t": "setChannelPassword", "channelId": id, "channelPassword": ""})
	su.until("channelPassword")
	waitEvent(t, obs, func(e mumble.ChannelEvent) bool { return e.Channel.ID == id && !e.Channel.EnterRestricted })
}

// TestNameTakeover: Mumble lets a same-IP newcomer take over an unregistered name and
// kicks the holder; the gateway must only allow that for the same certificate.
func TestNameTakeover(t *testing.T) {
	addr := mumbleAddr(t)
	srv := httptest.NewServer(New(Config{MumbleAddr: addr}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	name := "ayni-" + strconv.FormatInt(time.Now().UnixNano()%1e6, 36)

	obs, err := mumble.Dial(context.Background(), mumble.Config{Addr: addr, Username: "gozcu-" + name})
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Close()

	a := dialBridge(t, url, map[string]any{"username": name, "cert": nil})
	defer a.ws.CloseNow()
	cert := a.until("cert")
	as := uint32(a.until("ready")["session"].(float64))

	// The gateway's hash must match Mumble's.
	parsed, err := certs.Load([]byte(cert["pem"].(string)), []byte(cert["key"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	joined := waitEvent(t, obs, func(e mumble.UserEvent) bool { return e.User.Session == as })
	if joined.User.Hash != certHash(parsed) {
		t.Fatalf("hash mismatch: mumble %q gateway %q", joined.User.Hash, certHash(parsed))
	}

	// Another certificate, same name (different case): refused, the holder stays.
	b := dialBridge(t, url, map[string]any{"username": strings.ToUpper(name), "cert": nil})
	if m := b.until("reject"); m["reason"] != "UsernameInUse" {
		t.Fatalf("reject %v", m)
	}
	b.ws.CloseNow()
	a.send(map[string]any{"t": "ping", "ts": 1})
	a.until("pong")
	if _, ok := obs.State().User(as); !ok {
		t.Fatal("holder was kicked")
	}

	// Same certificate (reconnect from another tab/device): allowed, old session replaced.
	c := dialBridge(t, url, map[string]any{"username": name, "cert": map[string]any{"pem": cert["pem"], "key": cert["key"]}})
	defer c.ws.CloseNow()
	c.until("ready")
	waitEvent(t, obs, func(e mumble.UserRemoveEvent) bool { return e.Session == as })
}
