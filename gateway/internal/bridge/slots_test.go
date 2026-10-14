package bridge

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAcquireSlot(t *testing.T) {
	h := New(Config{MaxSessions: 4, ReservedSlots: 2, ReservedPassword: "vip"})
	if !h.ReservedSlotsEnabled() {
		t.Fatal("reserved slots should be on")
	}
	var rel []func()
	take := func(priv bool) bool {
		r, ok := h.acquireSlot(priv)
		if ok {
			rel = append(rel, r)
		}
		return ok
	}
	// 4 slots, 2 reserved: two general slots.
	if !take(false) || !take(false) {
		t.Fatal("general slots should be free")
	}
	if take(false) {
		t.Fatal("a normal session took a reserved slot")
	}
	if !take(true) || !take(true) {
		t.Fatal("privileged sessions should get the reserved slots")
	}
	if take(true) {
		t.Fatal("more sessions than MaxSessions")
	}
	// Freeing a reserved slot does not open it to normal sessions; freeing it twice is harmless.
	rel[2]()
	rel[2]()
	if take(false) {
		t.Fatal("a freed reserved slot went to a normal session")
	}
	if !take(true) {
		t.Fatal("freed reserved slot not reusable")
	}
	// A privileged session that finds a free general slot uses it, keeping reserved ones free.
	rel[0]()
	if !take(true) || take(true) {
		t.Fatal("privileged session should refill the general slot, then be full")
	}
}

func TestReservedSlotsOffWithoutPasswordOrRoom(t *testing.T) {
	if New(Config{MaxSessions: 4, ReservedSlots: 2}).ReservedSlotsEnabled() {
		t.Fatal("on without a password")
	}
	if New(Config{MaxSessions: 2, ReservedSlots: 2, ReservedPassword: "x"}).ReservedSlotsEnabled() {
		t.Fatal("on although no general slot is left")
	}
	if New(Config{ReservedSlots: 2, ReservedPassword: "x"}).ReservedSlotsEnabled() {
		t.Fatal("on without a session limit")
	}
}

func TestReservedPasswordHello(t *testing.T) {
	accessFailDelay = time.Millisecond
	t.Cleanup(func() { accessFailDelay = time.Second })

	h := New(Config{MumbleAddr: "127.0.0.1:1", MaxSessions: 2, ReservedSlots: 1, ReservedPassword: "vip"})
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	if m := helloReply(t, url, map[string]any{"username": "a", "reservedPassword": "nope"}); m["reason"] != rejectReservedWrong {
		t.Fatalf("wrong reserved password: %v", m)
	}
	// Right password, or none at all while a general slot is free: past the checks (Mumble is unreachable).
	for _, extra := range []map[string]any{{"reservedPassword": "vip"}, {}} {
		msg := map[string]any{"username": "a"}
		for k, v := range extra {
			msg[k] = v
		}
		if m := helloReply(t, url, msg); m["t"] != "error" || m["code"] != errCodeMumbleUnavailable {
			t.Fatalf("hello %v: %v", extra, m)
		}
	}
	// A full general area refuses normal sessions, but not privileged ones.
	rel, ok := h.acquireSlot(false)
	if !ok {
		t.Fatal("setup")
	}
	defer rel()
	if m := helloReply(t, url, map[string]any{"username": "a"}); m["reason"] != rejectWebFull {
		t.Fatalf("normal session on full general slots: %v", m)
	}
	if m := helloReply(t, url, map[string]any{"username": "a", "reservedPassword": "vip"}); m["code"] != errCodeMumbleUnavailable {
		t.Fatalf("reserved session: %v", m)
	}
	// Lockout after repeated wrong guesses.
	for i := 0; i < accessMaxFailures; i++ {
		helloReply(t, url, map[string]any{"username": "a", "reservedPassword": "bad"})
	}
	if m := helloReply(t, url, map[string]any{"username": "a", "reservedPassword": "vip"}); m["reason"] != rejectReservedRateLimited {
		t.Fatalf("lockout: %v", m)
	}
}
