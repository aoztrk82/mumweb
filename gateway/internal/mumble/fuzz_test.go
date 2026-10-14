package mumble

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

// Everything below parses bytes that come from the Mumble server (or, for relayed messages, from
// other Mumble users). None of it may panic.

func FuzzReadFrame(f *testing.F) {
	f.Add([]byte{0, 11, 0, 0, 0, 2, 0x0a, 0x00})
	f.Add([]byte{0, 1, 0, 0, 0, 3, 0x20, 1, 2})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, b []byte) {
		r := bytes.NewReader(b)
		for {
			typ, payload, err := readFrame(r)
			if err != nil {
				return
			}
			if msg := newMessage(typ); msg != nil {
				_ = proto.Unmarshal(payload, msg)
			}
			if typ == MsgUDPTunnel {
				_, _ = decodeAudio(payload)
			}
		}
	})
}

func FuzzDecodeAudio(f *testing.F) {
	f.Add([]byte{0x20, 0x01, 0x02, 0x03})
	f.Add([]byte{0x80, 0x00})
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = decodeAudio(b) })
}

// FuzzStateMessages feeds arbitrary protobuf into the state merging and the admin conversions.
func FuzzStateMessages(f *testing.F) {
	seed := func(m proto.Message) {
		b, _ := proto.Marshal(m)
		f.Add(b)
	}
	seed(&pb.UserState{Session: proto.Uint32(1), Name: proto.String("a"), ChannelId: proto.Uint32(1)})
	seed(&pb.ChannelState{ChannelId: proto.Uint32(1), Parent: proto.Uint32(0), Name: proto.String("c"), Links: []uint32{1, 2}})
	f.Fuzz(func(t *testing.T, b []byte) {
		st := newState()
		var cs pb.ChannelState
		if proto.Unmarshal(b, &cs) == nil {
			st.applyChannel(&cs)
		}
		var us pb.UserState
		if proto.Unmarshal(b, &us) == nil {
			st.applyUser(&us)
			st.removeUser(us.GetSession())
		}
		var acl pb.ACL
		if proto.Unmarshal(b, &acl) == nil {
			_ = channelACL(&acl)
			_ = aclMessage(1, channelACL(&acl))
		}
		var bl pb.BanList
		if proto.Unmarshal(b, &bl) == nil {
			_ = banEntries(&bl)
		}
		var ul pb.UserList
		if proto.Unmarshal(b, &ul) == nil {
			_ = userListEvent(&ul)
		}
	})
}
