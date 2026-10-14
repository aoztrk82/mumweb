package bridge

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
)

// The admin panel (/admin) is a normal bridge session with hello role "admin". It needs
// MUMWEB_ADMIN_PASSWORD, may log in as SuperUser, carries no audio and may send the
// admin* commands below. The Mumble server still enforces permissions for every action.

const (
	roleAdmin = "admin"

	rejectAdminDisabled = "AdminDisabled"
	rejectAdminWrong    = "AdminPasswordWrong"
	rejectAdminRequired = "AdminPasswordRequired"
	rejectAdminLimited  = "AdminRateLimited"

	// The server drops messages beyond ~1/s (burst 5) without telling anyone, queries
	// included. Admin commands are paced a little below that; a command that has to wait
	// longer than adminPaceMax is dropped with an adminBusy message.
	adminPerS    = 0.8
	adminBurst   = 4
	adminPaceMax = 6 * time.Second

	maxBanEntries  = 2000
	maxACLEntries  = 256
	maxACLGroups   = 256
	maxGroupUsers  = 4096
	maxGroupName   = 255
	maxBanText     = 512
	maxUserUpdates = 100
	maxPermBits    = 0x1fffff
)

// checkAdmin enforces MUMWEB_ADMIN_PASSWORD with its own lockout tracker.
func (s *session) checkAdmin(ctx context.Context, given string) (reason, msg string) {
	want := s.h.cfg.AdminPassword
	if want == "" {
		s.log.Warn("admin login refused (MUMWEB_ADMIN_PASSWORD is not set)")
		return rejectAdminDisabled, "the admin panel is disabled"
	}
	now := time.Now()
	if s.h.adminFails.locked(s.ip, now) {
		s.log.Warn("admin password: ip locked out")
		return rejectAdminLimited, "too many wrong admin passwords, try again later"
	}
	if given == "" {
		return rejectAdminRequired, "admin password required"
	}
	if passwordMatches(given, want) {
		s.h.adminFails.ok(s.ip)
		return "", ""
	}
	s.h.adminFails.fail(s.ip, now)
	s.log.Warn("admin password: wrong password")
	select {
	case <-time.After(accessFailDelay):
	case <-ctx.Done():
	}
	return rejectAdminWrong, "wrong admin password"
}

// audit logs the state-changing commands of an admin session. It records the command
// and the ids it targets, never passwords, tokens, chat text or ACL group names (a
// "#name" group is a channel password).
func (s *session) audit(m clientMsg) {
	switch m.T {
	case "ping", "queryPermissions", "requestDescription", "requestComment", "userStats", "queryChannelPassword",
		"adminBanList", "adminUserList", "adminACL", "setSelf", "setTokens", "listen", "text":
		return
	}
	args := []any{"action", m.T}
	if m.ChannelID != nil {
		args = append(args, "channel", *m.ChannelID)
	}
	if m.Session != nil {
		args = append(args, "target", *m.Session)
	}
	switch m.T {
	case "adminSetBanList":
		args = append(args, "bans", len(m.Bans))
	case "adminUpdateUsers":
		args = append(args, "updates", len(m.UserUpdates))
	case "adminSetACL":
		if m.ACL != nil {
			args = append(args, "acls", len(m.ACL.ACLs), "groups", len(m.ACL.Groups))
		}
	}
	s.log.Info("admin action", args...)
}

// adminPace waits until the session may send another message to Mumble. It reports
// false if that takes longer than adminPaceMax or the session ends.
func (s *session) adminPace(ctx context.Context) bool {
	deadline := time.Now().Add(adminPaceMax)
	for {
		now := time.Now()
		if s.adminBucket.take(now, adminPerS, adminBurst) {
			return true
		}
		if now.After(deadline) {
			return false
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return false
		}
	}
}

func (s *session) adminAction(ctx context.Context, m clientMsg) error {
	switch m.T {
	case "adminBanList":
		return s.mc.QueryBanList()
	case "adminUserList":
		return s.mc.QueryUserList()
	case "adminACL":
		if m.ChannelID == nil {
			return fmt.Errorf("%w: adminACL without channelId", errProtocol)
		}
		return s.mc.QueryACL(*m.ChannelID)
	}

	switch m.T {
	case "adminSetBanList":
		bans, err := s.checkBans(m.Bans)
		if err != nil {
			return err
		}
		return s.mc.SetBanList(bans)
	case "adminUpdateUsers":
		ups, err := checkUserUpdates(m.UserUpdates)
		if err != nil {
			return err
		}
		return s.mc.UpdateUserList(ups)
	case "adminSetACL":
		if m.ChannelID == nil || m.ACL == nil {
			return fmt.Errorf("%w: adminSetACL without channelId or acl", errProtocol)
		}
		acl, err := checkACL(*m.ChannelID, *m.ACL)
		if err != nil {
			return err
		}
		return s.mc.SetACL(acl)
	}
	return nil
}

func banKey(address string, mask uint32, hash string) string {
	if ip := net.ParseIP(address); ip != nil {
		address = ip.String()
	}
	return fmt.Sprintf("%s/%d/%s", address, mask, hash)
}

// rememberBans keeps the keys of the last ban list the server sent, so a later write
// may only keep those entries or add certificate-only ones.
func (s *session) rememberBans(bans []mumble.BanEntry) {
	if !s.admin {
		return
	}
	keys := make(map[string]bool, len(bans))
	for _, b := range bans {
		keys[banKey(b.Address, b.Mask, b.Hash)] = true
	}
	s.bansMu.Lock()
	s.lastBans = keys
	s.bansMu.Unlock()
}

// checkBans validates a replacement ban list. Existing entries (any kind, so the admin
// can remove IP bans made from the desktop client) stay as they are; new ones must be
// certificate-only. A new IP ban would hit every web user, who all share the gateway's address.
func (s *session) checkBans(in []banEntryJSON) ([]mumble.BanEntry, error) {
	if len(in) > maxBanEntries {
		return nil, fmt.Errorf("%w: too many ban entries", errProtocol)
	}
	s.bansMu.Lock()
	known := s.lastBans
	s.bansMu.Unlock()
	if known == nil {
		return nil, fmt.Errorf("%w: ban list written before it was read", errProtocol)
	}
	out := make([]mumble.BanEntry, 0, len(in))
	for _, b := range in {
		if net.ParseIP(b.Address) == nil || b.Mask > 128 {
			return nil, fmt.Errorf("%w: bad ban entry", errProtocol)
		}
		for _, v := range []string{b.Name, b.Hash, b.Reason, b.Start} {
			if len(v) > maxBanText || !utf8.ValidString(v) {
				return nil, fmt.Errorf("%w: bad ban text", errProtocol)
			}
		}
		if !known[banKey(b.Address, b.Mask, b.Hash)] {
			certOnly := net.ParseIP(b.Address).Equal(net.ParseIP(mumble.CertOnlyAddress)) && b.Mask == mumble.CertOnlyMask && b.Hash != ""
			if !certOnly {
				return nil, fmt.Errorf("%w: new ban entries must be certificate-only", errProtocol)
			}
		}
		out = append(out, mumble.BanEntry(b))
	}
	return out, nil
}

func checkUserUpdates(in []userUpdateJSON) ([]mumble.RegisteredUserUpdate, error) {
	if len(in) == 0 || len(in) > maxUserUpdates {
		return nil, fmt.Errorf("%w: bad user update count", errProtocol)
	}
	out := make([]mumble.RegisteredUserUpdate, 0, len(in))
	for _, u := range in {
		if u.ID == 0 { // SuperUser
			return nil, fmt.Errorf("%w: cannot change SuperUser", errProtocol)
		}
		if u.Name != nil {
			name := strings.TrimSpace(*u.Name)
			if name == "" || len(name) > maxUsernameLen || !utf8.ValidString(name) {
				return nil, fmt.Errorf("%w: bad user name", errProtocol)
			}
			u.Name = &name
		}
		out = append(out, mumble.RegisteredUserUpdate{ID: u.ID, Name: u.Name})
	}
	return out, nil
}

func checkACL(channel uint32, in aclJSON) (mumble.ChannelACL, error) {
	if len(in.ACLs) > maxACLEntries || len(in.Groups) > maxACLGroups {
		return mumble.ChannelACL{}, fmt.Errorf("%w: ACL too large", errProtocol)
	}
	out := mumble.ChannelACL{ChannelID: channel, InheritACLs: in.InheritACLs}
	for _, e := range in.ACLs {
		if e.Inherited {
			continue // read only; the server keeps the parent's entries
		}
		if (e.UserID == nil) == (e.Group == "") || len(e.Group) > maxGroupName || !utf8.ValidString(e.Group) ||
			e.Grant&^maxPermBits != 0 || e.Deny&^maxPermBits != 0 {
			return mumble.ChannelACL{}, fmt.Errorf("%w: bad ACL entry", errProtocol)
		}
		out.ACLs = append(out.ACLs, mumble.ACLEntry(e))
	}
	for _, g := range in.Groups {
		if g.Name == "" || len(g.Name) > maxGroupName || !utf8.ValidString(g.Name) ||
			len(g.Add) > maxGroupUsers || len(g.Remove) > maxGroupUsers {
			return mumble.ChannelACL{}, fmt.Errorf("%w: bad ACL group", errProtocol)
		}
		out.Groups = append(out.Groups, mumble.ACLGroup{Name: g.Name, Inherited: g.Inherited, Inherit: g.Inherit,
			Inheritable: g.Inheritable, Add: g.Add, Remove: g.Remove})
	}
	return out, nil
}
