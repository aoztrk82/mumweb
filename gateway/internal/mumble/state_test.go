package mumble

import (
	"reflect"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

func TestApplyUserPartial(t *testing.T) {
	s := newState()
	s.applyUser(&pb.UserState{
		Session:   proto.Uint32(7),
		Name:      proto.String("Mehmet"),
		ChannelId: proto.Uint32(3),
		SelfMute:  proto.Bool(true),
	})
	// Partial update: only self_deaf changes; other fields must be kept.
	u, ok := s.applyUser(&pb.UserState{Session: proto.Uint32(7), SelfDeaf: proto.Bool(true)})
	if !ok {
		t.Fatal("not applied")
	}
	want := User{Session: 7, Name: "Mehmet", ChannelID: 3, SelfMute: true, SelfDeaf: true}
	if !reflect.DeepEqual(u, want) {
		t.Fatalf("got %+v, want %+v", u, want)
	}

	// Explicit false must override, and user_id marks the user registered.
	u, _ = s.applyUser(&pb.UserState{Session: proto.Uint32(7), SelfMute: proto.Bool(false), UserId: proto.Uint32(0)})
	if u.SelfMute || !u.Registered || u.UserID != 0 {
		t.Fatalf("got %+v", u)
	}

	if _, ok := s.applyUser(&pb.UserState{Name: proto.String("no session")}); ok {
		t.Fatal("update without session applied")
	}

	s.removeUser(7)
	if _, ok := s.User(7); ok {
		t.Fatal("user not removed")
	}
}

func TestNewUserDefaultsToRoot(t *testing.T) {
	s := newState()
	u, _ := s.applyUser(&pb.UserState{Session: proto.Uint32(1), Name: proto.String("a")})
	if u.ChannelID != 0 {
		t.Fatalf("channel = %d", u.ChannelID)
	}
}

func TestApplyChannelPartialAndLinks(t *testing.T) {
	s := newState()
	s.applyChannel(&pb.ChannelState{
		ChannelId: proto.Uint32(3),
		Parent:    proto.Uint32(0),
		Name:      proto.String("Oyun"),
		Links:     []uint32{1, 2},
	})
	c, _, _ := s.applyChannel(&pb.ChannelState{
		ChannelId:   proto.Uint32(3),
		Position:    proto.Int32(5),
		LinksAdd:    []uint32{4, 2},
		LinksRemove: []uint32{1},
	})
	if c.Name != "Oyun" || c.Position != 5 || !slices.Equal(c.Links, []uint32{2, 4}) {
		t.Fatalf("got %+v", c)
	}

	// Returned values are copies, not views into the state.
	c.Links[0] = 99
	if got := s.Channels()[0].Links[0]; got != 2 {
		t.Fatalf("state mutated through copy: %d", got)
	}

	s.removeChannel(3)
	if len(s.Channels()) != 0 {
		t.Fatal("channel not removed")
	}
}

func TestListeningChannels(t *testing.T) {
	s := newState()
	s.applyUser(&pb.UserState{Session: proto.Uint32(1), ListeningChannelAdd: []uint32{3, 4}})
	u, _ := s.applyUser(&pb.UserState{Session: proto.Uint32(1), ListeningChannelAdd: []uint32{4, 5}, ListeningChannelRemove: []uint32{3}})
	if !slices.Equal(u.Listening, []uint32{4, 5}) {
		t.Fatalf("listening %v", u.Listening)
	}
	s.applyChannel(&pb.ChannelState{ChannelId: proto.Uint32(4)})
	s.removeChannel(4)
	if got, _ := s.User(1); !slices.Equal(got.Listening, []uint32{5}) {
		t.Fatalf("after channel removal %v", got.Listening)
	}
}

func TestLinksAreSymmetric(t *testing.T) {
	s := newState()
	for _, id := range []uint32{0, 1, 2} {
		s.applyChannel(&pb.ChannelState{ChannelId: proto.Uint32(id)})
	}
	_, others, _ := s.applyChannel(&pb.ChannelState{ChannelId: proto.Uint32(0), LinksAdd: []uint32{1, 2}})
	if len(others) != 2 {
		t.Fatalf("others %+v", others)
	}
	byID := func(id uint32) Channel {
		for _, c := range s.Channels() {
			if c.ID == id {
				return c
			}
		}
		return Channel{}
	}
	if !slices.Equal(byID(1).Links, []uint32{0}) || !slices.Equal(byID(2).Links, []uint32{0}) {
		t.Fatalf("not mirrored: %+v", s.Channels())
	}
	// A full links list replacing the old one also unlinks the other side.
	s.applyChannel(&pb.ChannelState{ChannelId: proto.Uint32(0), Links: []uint32{2}})
	if len(byID(1).Links) != 0 || !slices.Equal(byID(2).Links, []uint32{0}) {
		t.Fatalf("after replace: %+v", s.Channels())
	}
}
