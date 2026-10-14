package bridge

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"
)

// Invite links (MUMWEB_INVITE_SECRET) let someone join without the web access password. A
// token is stateless: 8 bytes of Unix expiry followed by an HMAC-SHA256 of those bytes keyed
// by the secret, base64url-encoded. There is nothing stored server-side, so a token cannot be
// revoked individually before it expires; rotating MUMWEB_INVITE_SECRET invalidates all of
// them at once. An admin session mints one with adminCreateInvite (see admin.go).

const (
	rejectInviteInvalid = "InviteInvalid"
	rejectInviteExpired = "InviteExpired"

	// maxInviteTTL bounds how far in the future a minted invite may expire.
	maxInviteTTL = 30 * 24 * time.Hour
)

var errInviteMalformed = errors.New("bridge: malformed invite token")

// signInvite mints a token that is valid until expiresAt.
func signInvite(secret string, expiresAt time.Time) string {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, uint64(expiresAt.Unix()))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(append(payload, mac.Sum(nil)...))
}

// verifyInvite checks the signature and returns the token's expiry.
func verifyInvite(secret, token string) (time.Time, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 8+sha256.Size {
		return time.Time{}, errInviteMalformed
	}
	payload, sig := raw[:8], raw[8:]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	if subtle.ConstantTimeCompare(sig, mac.Sum(nil)) != 1 {
		return time.Time{}, errInviteMalformed
	}
	return time.Unix(int64(binary.BigEndian.Uint64(payload)), 0), nil
}

// checkInvite enforces an invite token sent in hello. Unlike passwords, a token is
// unguessable (a 256-bit MAC), so it gets no per-IP lockout tracker.
func (s *session) checkInvite(token string) (reason, msg string) {
	exp, err := verifyInvite(s.h.cfg.InviteSecret, token)
	if err != nil {
		s.log.Info("invite: invalid token")
		return rejectInviteInvalid, "invite link is invalid"
	}
	if time.Now().After(exp) {
		s.log.Info("invite: expired token")
		return rejectInviteExpired, "invite link has expired"
	}
	return "", ""
}
