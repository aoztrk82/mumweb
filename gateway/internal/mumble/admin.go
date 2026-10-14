package mumble

import (
	"errors"
	"fmt"
	"net"

	"google.golang.org/protobuf/proto"

	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

// This file holds the server-administration messages used by the web admin panel:
// ban list, registered users and full ACL/group editing. The server enforces
// permissions (Ban, Register, Write); denials arrive as DeniedEvent.

// --- ban list ---

// BanEntry is one entry of the server's ban list. Address is an IP (IPv4 entries are
// stored by the server as IPv4-mapped IPv6, with Mask counted in 128 bits).
type BanEntry struct {
	Address  string
	Mask     uint32
	Name     string
	Hash     string
	Reason   string
	Start    string
	Duration uint32 // seconds, 0 = permanent
}

// CertOnlyAddress and CertOnlyMask identify the bans mumweb creates: they never match
// a real peer, so only the certificate hash bans (see BanByCertificate).
const (
	CertOnlyAddress = "100::"
	CertOnlyMask    = 128
)

func banEntries(list *pb.BanList) []BanEntry {
	out := make([]BanEntry, 0, len(list.GetBans()))
	for _, b := range list.GetBans() {
		out = append(out, BanEntry{
			Address: ipString(b.GetAddress()), Mask: b.GetMask(), Name: b.GetName(), Hash: b.GetHash(),
			Reason: b.GetReason(), Start: b.GetStart(), Duration: b.GetDuration(),
		})
	}
	return out
}

func ipString(b []byte) string {
	if len(b) != 16 {
		return ""
	}
	ip := net.IP(b)
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}

// QueryBanList asks for the ban list; the answer arrives as BanListEvent.
func (c *Client) QueryBanList() error {
	c.banMu.Lock()
	c.banQueried = true
	c.banMu.Unlock()
	return c.send(&pb.BanList{Query: proto.Bool(true)})
}

// SetBanList replaces the server's ban list (needs the Ban permission).
func (c *Client) SetBanList(bans []BanEntry) error {
	out := &pb.BanList{Query: proto.Bool(false)}
	for _, b := range bans {
		ip := net.ParseIP(b.Address)
		if ip == nil {
			return fmt.Errorf("mumble: bad ban address %q", b.Address)
		}
		if b.Mask > 128 {
			return fmt.Errorf("mumble: bad ban mask %d", b.Mask)
		}
		e := &pb.BanList_BanEntry{Address: ip.To16(), Mask: proto.Uint32(b.Mask), Duration: proto.Uint32(b.Duration)}
		if b.Name != "" {
			e.Name = proto.String(b.Name)
		}
		if b.Hash != "" {
			e.Hash = proto.String(b.Hash)
		}
		if b.Reason != "" {
			e.Reason = proto.String(b.Reason)
		}
		if b.Start != "" {
			e.Start = proto.String(b.Start)
		}
		out.Bans = append(out.Bans, e)
	}
	return c.send(out)
}

// --- registered users ---

// RegisteredUser is an entry of the server's user registry.
type RegisteredUser struct {
	ID          uint32
	Name        string
	LastSeen    string
	LastChannel *uint32
}

func userListEvent(l *pb.UserList) Event {
	out := make([]RegisteredUser, 0, len(l.GetUsers()))
	for _, u := range l.GetUsers() {
		r := RegisteredUser{ID: u.GetUserId(), Name: u.GetName(), LastSeen: u.GetLastSeen()}
		if u.LastChannel != nil {
			ch := u.GetLastChannel()
			r.LastChannel = &ch
		}
		out = append(out, r)
	}
	return UserListEvent{Users: out}
}

// QueryUserList asks for the registered users (needs Register on the root channel);
// the answer arrives as UserListEvent.
func (c *Client) QueryUserList() error { return c.send(&pb.UserList{}) }

// RegisteredUserUpdate renames (Name set) or deletes (Name nil) a registered user.
type RegisteredUserUpdate struct {
	ID   uint32
	Name *string
}

// UpdateUserList applies renames and deletions. An empty update would be read by the
// server as a query, so it is refused here.
func (c *Client) UpdateUserList(updates []RegisteredUserUpdate) error {
	if len(updates) == 0 {
		return errors.New("mumble: empty user list update")
	}
	out := &pb.UserList{}
	for _, u := range updates {
		e := &pb.UserList_User{UserId: proto.Uint32(u.ID)}
		if u.Name != nil {
			e.Name = proto.String(*u.Name)
		}
		out.Users = append(out.Users, e)
	}
	return c.send(out)
}

// --- ACL and groups ---

// ACLEntry is one access-control entry; exactly one of UserID and Group is set.
type ACLEntry struct {
	ApplyHere bool
	ApplySubs bool
	Inherited bool // read only: defined in a parent channel
	UserID    *uint32
	Group     string
	Grant     uint32
	Deny      uint32
}

// ACLGroup is a channel group.
type ACLGroup struct {
	Name             string
	Inherited        bool // read only: defined in a parent channel
	Inherit          bool
	Inheritable      bool
	Add              []uint32
	Remove           []uint32
	InheritedMembers []uint32 // read only
}

// ChannelACL is a channel's complete ACL as the server reports it.
type ChannelACL struct {
	ChannelID   uint32
	InheritACLs bool
	ACLs        []ACLEntry
	Groups      []ACLGroup
}

func channelACL(m *pb.ACL) ChannelACL {
	out := ChannelACL{ChannelID: m.GetChannelId(), InheritACLs: m.GetInheritAcls()}
	for _, a := range m.GetAcls() {
		e := ACLEntry{ApplyHere: a.GetApplyHere(), ApplySubs: a.GetApplySubs(), Inherited: a.GetInherited(),
			Group: a.GetGroup(), Grant: a.GetGrant(), Deny: a.GetDeny()}
		if a.UserId != nil {
			id := a.GetUserId()
			e.UserID = &id
		}
		out.ACLs = append(out.ACLs, e)
	}
	for _, g := range m.GetGroups() {
		out.Groups = append(out.Groups, ACLGroup{Name: g.GetName(), Inherited: g.GetInherited(), Inherit: g.GetInherit(),
			Inheritable: g.GetInheritable(), Add: g.Add, Remove: g.Remove, InheritedMembers: g.InheritedMembers})
	}
	return out
}

// QueryACL asks for a channel's ACL (needs Write); the answer arrives as ACLEvent.
func (c *Client) QueryACL(channelID uint32) error {
	c.aclReportMu.Lock()
	if c.aclReport == nil {
		c.aclReport = map[uint32]bool{}
	}
	c.aclReport[channelID] = true
	c.aclReportMu.Unlock()
	return c.send(&pb.ACL{ChannelId: &channelID, Query: proto.Bool(true)})
}

// reportACL turns an ACL reply into an ACLEvent if QueryACL asked for it.
func (c *Client) reportACL(m *pb.ACL) Event {
	c.aclReportMu.Lock()
	want := c.aclReport[m.GetChannelId()]
	delete(c.aclReport, m.GetChannelId())
	c.aclReportMu.Unlock()
	if !want {
		return nil
	}
	return ACLEvent{ACL: channelACL(m)}
}

// SetACL replaces a channel's own ACL entries and groups (needs Write). Inherited
// entries and groups that only mirror a parent's are not sent, like the desktop client.
func (c *Client) SetACL(a ChannelACL) error {
	id := a.ChannelID
	return c.send(aclMessage(id, a))
}

func aclMessage(id uint32, a ChannelACL) *pb.ACL {
	out := &pb.ACL{ChannelId: &id, InheritAcls: proto.Bool(a.InheritACLs)}
	for _, e := range a.ACLs {
		if e.Inherited || (e.UserID == nil && e.Group == "") {
			continue
		}
		p := &pb.ACL_ChanACL{ApplyHere: proto.Bool(e.ApplyHere), ApplySubs: proto.Bool(e.ApplySubs),
			Grant: proto.Uint32(e.Grant), Deny: proto.Uint32(e.Deny)}
		if e.UserID != nil {
			p.UserId = proto.Uint32(*e.UserID)
		} else {
			p.Group = proto.String(e.Group)
		}
		out.Acls = append(out.Acls, p)
	}
	for _, g := range a.Groups {
		if g.Inherited && g.Inherit && g.Inheritable && len(g.Add) == 0 && len(g.Remove) == 0 {
			continue
		}
		out.Groups = append(out.Groups, &pb.ACL_ChanGroup{Name: proto.String(g.Name), Inherit: proto.Bool(g.Inherit),
			Inheritable: proto.Bool(g.Inheritable), Add: g.Add, Remove: g.Remove})
	}
	return out
}
