// Package mumble implements the client side of the Mumble TCP control protocol
// (TLS, framing, handshake, state tracking) and voice over the UDPTunnel message.
package mumble

import (
	"encoding/binary"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"

	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

// MsgType is the 16-bit message type prefix of a TCP frame.
// Values follow the message order in Mumble.proto.
type MsgType uint16

const (
	MsgVersion                MsgType = 0
	MsgUDPTunnel              MsgType = 1
	MsgAuthenticate           MsgType = 2
	MsgPing                   MsgType = 3
	MsgReject                 MsgType = 4
	MsgServerSync             MsgType = 5
	MsgChannelRemove          MsgType = 6
	MsgChannelState           MsgType = 7
	MsgUserRemove             MsgType = 8
	MsgUserState              MsgType = 9
	MsgBanList                MsgType = 10
	MsgTextMessage            MsgType = 11
	MsgPermissionDenied       MsgType = 12
	MsgACL                    MsgType = 13
	MsgQueryUsers             MsgType = 14
	MsgCryptSetup             MsgType = 15
	MsgContextActionModify    MsgType = 16
	MsgContextAction          MsgType = 17
	MsgUserList               MsgType = 18
	MsgVoiceTarget            MsgType = 19
	MsgPermissionQuery        MsgType = 20
	MsgCodecVersion           MsgType = 21
	MsgUserStats              MsgType = 22
	MsgRequestBlob            MsgType = 23
	MsgServerConfig           MsgType = 24
	MsgSuggestConfig          MsgType = 25
	MsgPluginDataTransmission MsgType = 26
)

var msgTypeNames = [...]string{
	"Version", "UDPTunnel", "Authenticate", "Ping", "Reject", "ServerSync",
	"ChannelRemove", "ChannelState", "UserRemove", "UserState", "BanList",
	"TextMessage", "PermissionDenied", "ACL", "QueryUsers", "CryptSetup",
	"ContextActionModify", "ContextAction", "UserList", "VoiceTarget",
	"PermissionQuery", "CodecVersion", "UserStats", "RequestBlob",
	"ServerConfig", "SuggestConfig", "PluginDataTransmission",
}

func (t MsgType) String() string {
	if int(t) < len(msgTypeNames) {
		return msgTypeNames[t]
	}
	return fmt.Sprintf("MsgType(%d)", uint16(t))
}

const (
	frameHeaderSize = 6
	// MaxPayloadSize mirrors the server's limit on a single TCP message (0x7fffff bytes).
	MaxPayloadSize = 0x7fffff
)

// writeFrame writes one framed message as a single Write call.
func writeFrame(w io.Writer, t MsgType, payload []byte) error {
	if len(payload) > MaxPayloadSize {
		return fmt.Errorf("mumble: %s payload too large (%d bytes)", t, len(payload))
	}
	buf := make([]byte, frameHeaderSize+len(payload))
	binary.BigEndian.PutUint16(buf[0:2], uint16(t))
	binary.BigEndian.PutUint32(buf[2:6], uint32(len(payload)))
	copy(buf[frameHeaderSize:], payload)
	_, err := w.Write(buf)
	return err
}

// readFrame reads one framed message. The returned payload is freshly allocated.
func readFrame(r io.Reader) (MsgType, []byte, error) {
	var hdr [frameHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	t := MsgType(binary.BigEndian.Uint16(hdr[0:2]))
	n := binary.BigEndian.Uint32(hdr[2:6])
	if n > MaxPayloadSize {
		return 0, nil, fmt.Errorf("mumble: %s frame too large (%d bytes)", t, n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return t, payload, nil
}

// newMessage returns an empty protobuf message for the types this client decodes,
// or nil for types it ignores (and for UDPTunnel, whose payload is a raw UDP packet).
func newMessage(t MsgType) proto.Message {
	switch t {
	case MsgVersion:
		return &pb.Version{}
	case MsgPing:
		return &pb.Ping{}
	case MsgReject:
		return &pb.Reject{}
	case MsgServerSync:
		return &pb.ServerSync{}
	case MsgChannelRemove:
		return &pb.ChannelRemove{}
	case MsgChannelState:
		return &pb.ChannelState{}
	case MsgUserRemove:
		return &pb.UserRemove{}
	case MsgUserState:
		return &pb.UserState{}
	case MsgTextMessage:
		return &pb.TextMessage{}
	case MsgPermissionDenied:
		return &pb.PermissionDenied{}
	case MsgCodecVersion:
		return &pb.CodecVersion{}
	case MsgServerConfig:
		return &pb.ServerConfig{}
	case MsgPermissionQuery:
		return &pb.PermissionQuery{}
	case MsgUserStats:
		return &pb.UserStats{}
	case MsgBanList:
		return &pb.BanList{}
	}
	return nil
}

// msgTypeOf maps an outgoing protobuf message to its frame type.
func msgTypeOf(m proto.Message) (MsgType, bool) {
	switch m.(type) {
	case *pb.Version:
		return MsgVersion, true
	case *pb.Authenticate:
		return MsgAuthenticate, true
	case *pb.Ping:
		return MsgPing, true
	case *pb.UserState:
		return MsgUserState, true
	case *pb.ChannelState:
		return MsgChannelState, true
	case *pb.TextMessage:
		return MsgTextMessage, true
	case *pb.PermissionQuery:
		return MsgPermissionQuery, true
	case *pb.UserRemove:
		return MsgUserRemove, true
	case *pb.ChannelRemove:
		return MsgChannelRemove, true
	case *pb.UserStats:
		return MsgUserStats, true
	case *pb.RequestBlob:
		return MsgRequestBlob, true
	case *pb.BanList:
		return MsgBanList, true
	}
	return 0, false
}
