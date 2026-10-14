package mumble

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

// adminClient connects a Client to a fake server and returns both.
func adminClient(t *testing.T) (*Client, *serverConn) {
	t.Helper()
	s := newFakeServer(t)
	done := make(chan *Client, 1)
	go func() {
		c, err := dialFake(t, s, Config{})
		if err != nil {
			t.Error(err)
		}
		done <- c
	}()
	sc := s.accept()
	sc.handshake()
	sc.sync(1)
	c := <-done
	if c == nil {
		t.FailNow()
	}
	t.Cleanup(func() { c.Close() })
	return c, sc
}

func TestBanListQueryAndSet(t *testing.T) {
	c, sc := adminClient(t)

	if err := c.QueryBanList(); err != nil {
		t.Fatal(err)
	}
	var q pb.BanList
	_ = proto.Unmarshal(sc.expect(MsgBanList), &q)
	if !q.GetQuery() {
		t.Fatalf("query = %v", &q)
	}
	v4 := append(make([]byte, 10), 0xff, 0xff, 1, 2, 3, 4)
	sc.send(MsgBanList, &pb.BanList{Bans: []*pb.BanList_BanEntry{
		{Address: v4, Mask: proto.Uint32(120), Name: proto.String("x"), Reason: proto.String("spam"), Duration: proto.Uint32(60)},
		{Address: banAddress, Mask: proto.Uint32(128), Hash: proto.String("abc")},
	}})
	ev := nextEvent(t, c).(BanListEvent)
	if len(ev.Bans) != 2 || ev.Bans[0].Address != "1.2.3.4" || ev.Bans[0].Mask != 120 || ev.Bans[0].Duration != 60 ||
		ev.Bans[1].Address != CertOnlyAddress || ev.Bans[1].Hash != "abc" {
		t.Fatalf("bans %+v", ev.Bans)
	}

	// A reply nobody asked for (the kick-ban flow) is not reported.
	sc.send(MsgBanList, &pb.BanList{})
	sc.send(MsgTextMessage, &pb.TextMessage{Message: proto.String("next")})
	if _, ok := nextEvent(t, c).(TextEvent); !ok {
		t.Fatal("unsolicited ban list produced an event")
	}

	if err := c.SetBanList(ev.Bans); err != nil {
		t.Fatal(err)
	}
	var w pb.BanList
	_ = proto.Unmarshal(sc.expect(MsgBanList), &w)
	if w.GetQuery() || len(w.Bans) != 2 || len(w.Bans[0].Address) != 16 || w.Bans[0].GetMask() != 120 || w.Bans[1].GetHash() != "abc" {
		t.Fatalf("write %v", &w)
	}
	if err := c.SetBanList([]BanEntry{{Address: "nope", Mask: 1}}); err == nil {
		t.Fatal("bad address accepted")
	}
	if err := c.SetBanList([]BanEntry{{Address: "1.2.3.4", Mask: 200}}); err == nil {
		t.Fatal("bad mask accepted")
	}
}

func TestUserList(t *testing.T) {
	c, sc := adminClient(t)

	if err := c.QueryUserList(); err != nil {
		t.Fatal(err)
	}
	var q pb.UserList
	_ = proto.Unmarshal(sc.expect(MsgUserList), &q)
	if len(q.Users) != 0 {
		t.Fatalf("query must be empty: %v", &q)
	}
	sc.send(MsgUserList, &pb.UserList{Users: []*pb.UserList_User{
		{UserId: proto.Uint32(1), Name: proto.String("ali"), LastSeen: proto.String("2026-01-01"), LastChannel: proto.Uint32(3)},
		{UserId: proto.Uint32(2), Name: proto.String("veli")},
	}})
	ev := nextEvent(t, c).(UserListEvent)
	if len(ev.Users) != 2 || ev.Users[0].Name != "ali" || ev.Users[0].LastChannel == nil || *ev.Users[0].LastChannel != 3 || ev.Users[1].LastChannel != nil {
		t.Fatalf("users %+v", ev.Users)
	}

	newName := "ayse"
	if err := c.UpdateUserList([]RegisteredUserUpdate{{ID: 1, Name: &newName}, {ID: 2}}); err != nil {
		t.Fatal(err)
	}
	var w pb.UserList
	_ = proto.Unmarshal(sc.expect(MsgUserList), &w)
	if len(w.Users) != 2 || w.Users[0].GetName() != "ayse" || w.Users[1].Name != nil || w.Users[1].GetUserId() != 2 {
		t.Fatalf("write %v", &w)
	}
	if err := c.UpdateUserList(nil); err == nil {
		t.Fatal("empty update would be read as a query")
	}
}

// aclReply is what a server sends for a channel with inherited and own entries.
func aclReply() *pb.ACL {
	return &pb.ACL{
		ChannelId:   proto.Uint32(5),
		InheritAcls: proto.Bool(true),
		Acls: []*pb.ACL_ChanACL{
			{ApplyHere: proto.Bool(true), ApplySubs: proto.Bool(true), Inherited: proto.Bool(true), Group: proto.String("all"), Grant: proto.Uint32(0x674), Deny: proto.Uint32(0)},
			{ApplyHere: proto.Bool(true), ApplySubs: proto.Bool(false), Inherited: proto.Bool(false), Group: proto.String("admin"), Grant: proto.Uint32(0x1f0fff), Deny: proto.Uint32(0)},
			{ApplyHere: proto.Bool(true), ApplySubs: proto.Bool(true), Inherited: proto.Bool(false), UserId: proto.Uint32(9), Grant: proto.Uint32(0), Deny: proto.Uint32(0x8)},
		},
		Groups: []*pb.ACL_ChanGroup{
			{Name: proto.String("inheritedOnly"), Inherited: proto.Bool(true), Inherit: proto.Bool(true), Inheritable: proto.Bool(true), InheritedMembers: []uint32{4}},
			{Name: proto.String("admin"), Inherited: proto.Bool(false), Inherit: proto.Bool(true), Inheritable: proto.Bool(false), Add: []uint32{1, 2}, Remove: []uint32{3}},
		},
	}
}

func TestACLRoundTrip(t *testing.T) {
	c, sc := adminClient(t)

	if err := c.QueryACL(5); err != nil {
		t.Fatal(err)
	}
	var q pb.ACL
	_ = proto.Unmarshal(sc.expect(MsgACL), &q)
	if q.GetChannelId() != 5 || !q.GetQuery() {
		t.Fatalf("query %v", &q)
	}
	sc.send(MsgACL, aclReply())
	ev := nextEvent(t, c).(ACLEvent).ACL
	if ev.ChannelID != 5 || !ev.InheritACLs || len(ev.ACLs) != 3 || len(ev.Groups) != 2 {
		t.Fatalf("event %+v", ev)
	}
	if !ev.ACLs[0].Inherited || ev.ACLs[2].UserID == nil || *ev.ACLs[2].UserID != 9 || ev.Groups[1].Remove[0] != 3 || ev.Groups[0].InheritedMembers[0] != 4 {
		t.Fatalf("event %+v", ev)
	}

	// Writing the unchanged ACL back must reproduce only the channel's own entries.
	if err := c.SetACL(ev); err != nil {
		t.Fatal(err)
	}
	var w pb.ACL
	_ = proto.Unmarshal(sc.expect(MsgACL), &w)
	if w.GetQuery() || !w.GetInheritAcls() || len(w.Acls) != 2 || len(w.Groups) != 1 {
		t.Fatalf("write %v", &w)
	}
	own := aclReply()
	if !proto.Equal(w.Acls[0], &pb.ACL_ChanACL{ApplyHere: own.Acls[1].ApplyHere, ApplySubs: own.Acls[1].ApplySubs, Group: own.Acls[1].Group, Grant: own.Acls[1].Grant, Deny: proto.Uint32(0)}) {
		t.Fatalf("acl 0 %v", w.Acls[0])
	}
	if w.Acls[1].GetUserId() != 9 || w.Acls[1].GetDeny() != 0x8 || w.Acls[1].Group != nil {
		t.Fatalf("acl 1 %v", w.Acls[1])
	}
	if g := w.Groups[0]; g.GetName() != "admin" || g.GetInheritable() || len(g.Add) != 2 || len(g.Remove) != 1 {
		t.Fatalf("group %v", g)
	}
}

// A password query and an admin ACL query on the same channel must both be answered.
func TestACLReportAlongsidePassword(t *testing.T) {
	c, sc := adminClient(t)
	if err := c.QueryChannelPassword(5); err != nil {
		t.Fatal(err)
	}
	if err := c.QueryACL(5); err != nil {
		t.Fatal(err)
	}
	sc.expect(MsgACL)
	sc.expect(MsgACL)
	sc.send(MsgACL, aclReply())
	got := map[string]bool{}
	for range 2 {
		switch nextEvent(t, c).(type) {
		case ACLEvent:
			got["acl"] = true
		case ChannelPasswordEvent:
			got["pw"] = true
		}
	}
	if !got["acl"] || !got["pw"] {
		t.Fatalf("events %v", got)
	}
}
