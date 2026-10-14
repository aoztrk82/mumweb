package bridge

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
)

func TestAdminDisabledByDefault(t *testing.T) {
	h := New(Config{MumbleAddr: "127.0.0.1:1"})
	if h.AdminEnabled() {
		t.Fatal("admin must be off without MUMWEB_ADMIN_PASSWORD")
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	if m := helloReply(t, url, map[string]any{"username": "SuperUser", "role": "admin", "adminPassword": ""}); m["reason"] != rejectAdminDisabled {
		t.Fatalf("empty admin password accepted: %v", m)
	}
}

func TestAdminPassword(t *testing.T) {
	accessFailDelay = time.Millisecond
	t.Cleanup(func() { accessFailDelay = time.Second })

	// A web access password is set too: admin sessions use the admin password instead.
	h := New(Config{MumbleAddr: "127.0.0.1:1", AccessPassword: "web", AdminPassword: "adm1n"})
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	if m := helloReply(t, url, map[string]any{"username": "SuperUser", "role": "admin"}); m["reason"] != rejectAdminRequired {
		t.Fatalf("missing: %v", m)
	}
	// The web access password and the Mumble password are not the admin password.
	if m := helloReply(t, url, map[string]any{"username": "SuperUser", "role": "admin", "adminPassword": "web", "accessPassword": "web", "password": "adm1n"}); m["reason"] != rejectAdminWrong {
		t.Fatalf("wrong: %v", m)
	}
	// Right password: SuperUser gets past the checks although MUMWEB_ALLOW_SUPERUSER is off
	// (and then fails to reach Mumble).
	if m := helloReply(t, url, map[string]any{"username": "SuperUser", "role": "admin", "adminPassword": "adm1n"}); m["t"] != "error" || m["code"] != errCodeMumbleUnavailable {
		t.Fatalf("right: %v", m)
	}
	// The main UI still refuses SuperUser.
	if m := helloReply(t, url, map[string]any{"username": "SuperUser", "accessPassword": "web"}); m["reason"] != "SuperUserDisabled" {
		t.Fatalf("main UI superuser: %v", m)
	}
	// Role without the admin password must not fall back to the access password.
	if m := helloReply(t, url, map[string]any{"username": "a", "role": "admin", "accessPassword": "web"}); m["reason"] != rejectAdminRequired {
		t.Fatalf("access password accepted for admin: %v", m)
	}

	// Admin lockout is separate: it must not lock the main UI out of the same IP.
	for i := 0; i < accessMaxFailures; i++ {
		helloReply(t, url, map[string]any{"username": "a", "role": "admin", "adminPassword": "bad"})
	}
	if m := helloReply(t, url, map[string]any{"username": "a", "role": "admin", "adminPassword": "adm1n"}); m["reason"] != rejectAdminLimited {
		t.Fatalf("admin lockout: %v", m)
	}
	if m := helloReply(t, url, map[string]any{"username": "a", "accessPassword": "web"}); m["t"] != "error" {
		t.Fatalf("admin failures locked the main UI: %v", m)
	}
}

func TestCheckBans(t *testing.T) {
	s := &session{}
	if _, err := s.checkBans(nil); !errors.Is(err, errProtocol) {
		t.Fatal("writing before reading must fail")
	}
	s.rememberBans([]mumble.BanEntry{{Address: "1.2.3.4", Mask: 128, Hash: "h1"}})
	s.admin = true
	s.rememberBans([]mumble.BanEntry{{Address: "1.2.3.4", Mask: 128, Hash: "h1"}})

	// Keeping and removing existing entries is fine, also IP bans.
	if out, err := s.checkBans([]banEntryJSON{{Address: "1.2.3.4", Mask: 128, Hash: "h1", Reason: "edited"}}); err != nil || len(out) != 1 {
		t.Fatalf("keep: %v %v", out, err)
	}
	if out, err := s.checkBans(nil); err != nil || len(out) != 0 {
		t.Fatalf("remove all: %v %v", out, err)
	}
	// A new certificate-only entry is fine.
	if _, err := s.checkBans([]banEntryJSON{{Address: "1.2.3.4", Mask: 128, Hash: "h1"}, {Address: "100::", Mask: 128, Hash: "h2"}}); err != nil {
		t.Fatalf("new cert ban: %v", err)
	}
	// A new IP ban (or a cert ban without hash) is not: it would hit every web user.
	for _, b := range []banEntryJSON{
		{Address: "10.0.0.1", Mask: 128, Hash: "x"},
		{Address: "100::", Mask: 128},
		{Address: "100::", Mask: 64, Hash: "x"},
		{Address: "bogus", Mask: 128, Hash: "x"},
		{Address: "100::", Mask: 129, Hash: "x"},
	} {
		if _, err := s.checkBans([]banEntryJSON{b}); !errors.Is(err, errProtocol) {
			t.Fatalf("accepted %+v", b)
		}
	}
}

func TestCheckACLAndUserUpdates(t *testing.T) {
	uid := uint32(4)
	ok := aclJSON{InheritACLs: true,
		ACLs: []aclEntryJSON{
			{Inherited: true, Group: "all"}, // dropped
			{ApplyHere: true, Group: "admin", Grant: 0x1f0fff},
			{ApplyHere: true, UserID: &uid, Deny: 0x8},
		},
		Groups: []aclGroupJSON{{Name: "admin", Inherit: true, Add: []uint32{1}}},
	}
	out, err := checkACL(3, ok)
	if err != nil || out.ChannelID != 3 || len(out.ACLs) != 2 || len(out.Groups) != 1 {
		t.Fatalf("ok acl: %+v %v", out, err)
	}
	for name, bad := range map[string]aclJSON{
		"both user and group": {ACLs: []aclEntryJSON{{UserID: &uid, Group: "x"}}},
		"neither":             {ACLs: []aclEntryJSON{{}}},
		"unknown bits":        {ACLs: []aclEntryJSON{{Group: "x", Grant: 1 << 30}}},
		"nameless group":      {Groups: []aclGroupJSON{{}}},
	} {
		if _, err := checkACL(1, bad); !errors.Is(err, errProtocol) {
			t.Fatalf("%s accepted", name)
		}
	}

	n := "  ali "
	ups, err := checkUserUpdates([]userUpdateJSON{{ID: 2, Name: &n}, {ID: 3}})
	if err != nil || *ups[0].Name != "ali" || ups[1].Name != nil {
		t.Fatalf("updates: %+v %v", ups, err)
	}
	empty := " "
	for name, bad := range map[string][]userUpdateJSON{
		"none": nil, "superuser": {{ID: 0}}, "empty name": {{ID: 2, Name: &empty}},
	} {
		if _, err := checkUserUpdates(bad); !errors.Is(err, errProtocol) {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestAdminPace(t *testing.T) {
	s := &session{}
	ctx := context.Background()
	start := time.Now()
	for i := 0; i < adminBurst; i++ {
		if !s.adminPace(ctx) {
			t.Fatalf("burst command %d refused", i)
		}
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("burst commands were delayed")
	}
	// The next command waits for a token (about 1/adminPerS seconds) instead of being dropped.
	if !s.adminPace(ctx) {
		t.Fatal("command beyond the burst was dropped instead of delayed")
	}
	if d := time.Since(start); d < time.Second {
		t.Fatalf("command beyond the burst was not paced (%v)", d)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if s.adminPace(cctx) {
		t.Fatal("pacing ignores a cancelled session")
	}
}

// TestAdminPanelEndToEnd drives an admin session against a real Mumble server as SuperUser:
// ban list, registered users and an ACL round trip. Needs MUMWEB_TEST_MUMBLE_ADDR and
// MUMWEB_TEST_SUPERUSER_PASSWORD.
func TestAdminPanelEndToEnd(t *testing.T) {
	addr := mumbleAddr(t)
	pw := os.Getenv("MUMWEB_TEST_SUPERUSER_PASSWORD")
	if pw == "" {
		t.Skip("MUMWEB_TEST_SUPERUSER_PASSWORD not set")
	}
	// AllowSuperUser stays off: the admin role alone must let SuperUser in.
	srv := httptest.NewServer(New(Config{MumbleAddr: addr, AdminPassword: "adm1n"}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	suffix := strconv.FormatInt(time.Now().UnixNano()%1e6, 36)

	// A normal session must not be able to use admin commands.
	user := dialBridge(t, url, map[string]any{"username": "gozcu-" + suffix})
	defer user.ws.CloseNow()
	user.until("ready")
	user.send(map[string]any{"t": "adminBanList"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, _, err := user.ws.Read(ctx); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("admin command on a normal session: %v", err)
	}
	cancel()

	adm := dialBridge(t, url, map[string]any{"username": "SuperUser", "password": pw, "role": "admin", "adminPassword": "adm1n"})
	defer adm.ws.CloseNow()
	adm.until("ready")

	// Registered users: a fresh server has none, but the answer must be a list.
	adm.send(map[string]any{"t": "adminUserList"})
	if m := adm.until("userList"); m["users"] == nil {
		t.Fatalf("userList %v", m)
	}

	// Ban list: add a certificate-only ban, see it, remove it again.
	hash := strings.Repeat("ab", 20)
	adm.send(map[string]any{"t": "adminBanList"})
	before := adm.until("banList")["bans"].([]any)
	added := append(before, map[string]any{"address": "100::", "mask": 128, "hash": hash, "name": "e2e-" + suffix, "reason": "test", "duration": 0})
	adm.send(map[string]any{"t": "adminSetBanList", "bans": added})
	adm.send(map[string]any{"t": "adminBanList"})
	after := adm.until("banList")["bans"].([]any)
	found := false
	for _, b := range after {
		if b.(map[string]any)["hash"] == hash {
			found = true
		}
	}
	if !found {
		t.Fatalf("ban not stored: %v", after)
	}
	adm.send(map[string]any{"t": "adminSetBanList", "bans": before})
	adm.send(map[string]any{"t": "adminBanList"})
	if now := adm.until("banList")["bans"].([]any); len(now) != len(before) {
		t.Fatalf("ban not removed: %v", now)
	}

	// ACL round trip on a scratch channel.
	adm.send(map[string]any{"t": "createChannel", "parent": 0, "name": "adm-" + suffix, "temporary": true})
	var ch uint32
	for ch == 0 {
		if m, _ := adm.next(); m != nil && m["t"] == "channel" && m["name"] == "adm-"+suffix {
			ch = uint32(m["id"].(float64))
		}
	}
	adm.send(map[string]any{"t": "adminACL", "channelId": ch})
	acl := adm.until("acl")
	if acl["channelId"] != float64(ch) || acl["inheritAcls"] != true {
		t.Fatalf("acl %v", acl)
	}
	own := []any{map[string]any{"applyHere": true, "applySubs": false, "group": "e2egroup", "grant": 4, "deny": 8}}
	groups := []any{map[string]any{"name": "e2egroup", "inherit": true, "inheritable": true, "add": []any{0}, "remove": []any{}}}
	adm.send(map[string]any{"t": "adminSetACL", "channelId": ch, "acl": map[string]any{"inheritAcls": true, "acls": own, "groups": groups}})
	adm.send(map[string]any{"t": "adminACL", "channelId": ch})
	acl = adm.until("acl")
	var gotEntry, gotGroup bool
	for _, e := range acl["acls"].([]any) {
		em := e.(map[string]any)
		if em["group"] == "e2egroup" && em["inherited"] == false && em["grant"] == 4.0 && em["deny"] == 8.0 && em["applySubs"] == false {
			gotEntry = true
		}
	}
	for _, g := range acl["groups"].([]any) {
		gm := g.(map[string]any)
		if gm["name"] == "e2egroup" && len(gm["add"].([]any)) == 1 {
			gotGroup = true
		}
	}
	if !gotEntry || !gotGroup {
		t.Fatalf("acl not stored: %v", acl)
	}

	// A new IP ban would hit every web user: the gateway refuses and ends the session.
	adm.send(map[string]any{"t": "adminSetBanList", "bans": []any{map[string]any{"address": "10.9.8.7", "mask": 128, "hash": hash, "duration": 0}}})
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := adm.ws.Read(ctx); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("new IP ban: %v", err)
			}
			break
		}
	}
}
