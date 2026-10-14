package bridge

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignVerifyInvite(t *testing.T) {
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	token := signInvite("s3cret", exp)
	got, err := verifyInvite("s3cret", token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !got.Equal(exp) {
		t.Fatalf("expiry mismatch: got %v want %v", got, exp)
	}
	if _, err := verifyInvite("other-secret", token); err == nil {
		t.Fatal("wrong secret accepted")
	}
	if _, err := verifyInvite("s3cret", token+"x"); err == nil {
		t.Fatal("tampered token accepted")
	}
	if _, err := verifyInvite("s3cret", "not-base64url!!"); err == nil {
		t.Fatal("malformed token accepted")
	}
}

func TestInviteLink(t *testing.T) {
	h := New(Config{MumbleAddr: "127.0.0.1:1", InviteSecret: "s3cret"})
	if !h.InviteEnabled() {
		t.Fatal("InviteEnabled should be true")
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")

	valid := signInvite("s3cret", time.Now().Add(time.Hour))
	if m := helloReply(t, url, map[string]any{"username": "a", "inviteToken": valid}); m["t"] != "error" || m["code"] != errCodeMumbleUnavailable {
		t.Fatalf("valid invite: %v", m)
	}

	expired := signInvite("s3cret", time.Now().Add(-time.Minute))
	if m := helloReply(t, url, map[string]any{"username": "a", "inviteToken": expired}); m["reason"] != rejectInviteExpired {
		t.Fatalf("expired invite: %v", m)
	}

	if m := helloReply(t, url, map[string]any{"username": "a", "inviteToken": "garbage"}); m["reason"] != rejectInviteInvalid {
		t.Fatalf("invalid invite: %v", m)
	}

	// An invite token bypasses the access password entirely when both are configured.
	h2 := New(Config{MumbleAddr: "127.0.0.1:1", InviteSecret: "s3cret", AccessPassword: "open-sesame"})
	srv2 := httptest.NewServer(h2)
	defer srv2.Close()
	url2 := "ws" + strings.TrimPrefix(srv2.URL, "http")
	if m := helloReply(t, url2, map[string]any{"username": "a", "inviteToken": valid}); m["t"] != "error" || m["code"] != errCodeMumbleUnavailable {
		t.Fatalf("invite should skip the access password: %v", m)
	}
	// No invite token sent: the access password is still required.
	if m := helloReply(t, url2, map[string]any{"username": "a"}); m["reason"] != rejectAccessRequired {
		t.Fatalf("missing invite falls back to access password: %v", m)
	}
}

func TestInviteDisabled(t *testing.T) {
	// A token is ignored (falls back to the access check) unless MUMWEB_INVITE_SECRET is set.
	h := New(Config{MumbleAddr: "127.0.0.1:1", AccessPassword: "open-sesame"})
	srv := httptest.NewServer(h)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	token := signInvite("whatever", time.Now().Add(time.Hour))
	if m := helloReply(t, url, map[string]any{"username": "a", "inviteToken": token}); m["reason"] != rejectAccessRequired {
		t.Fatalf("invite token should be ignored when disabled: %v", m)
	}
}
