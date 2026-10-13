package mumble

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	udp "github.com/aoztrk82/mumweb/gateway/proto/mumbleudp"
)

// UDP packet header bytes for the protobuf format (Mumble >= 1.5).
// Legacy packets instead start with (codec<<5)|target, e.g. 0x80 for Opus.
const (
	udpTypeAudio byte = 0
	udpTypePing  byte = 1
)

// ErrLegacyAudio is returned for voice packets in the pre-1.5 binary format.
var ErrLegacyAudio = errors.New("mumble: legacy audio packet format")

// AudioPacket is one Opus voice packet, as carried by MumbleUDP.Audio.
type AudioPacket struct {
	// Target is set by the sender (0 = normal talking).
	Target uint32
	// Context is set by the server on received audio (0 = normal speech,
	// 1 = shout, 2 = whisper, 3 = channel listener).
	Context          uint32
	Sender           uint32 // sender session, set on received audio
	FrameNumber      uint64
	Opus             []byte
	VolumeAdjustment float32 // 0 = unset
	Terminator       bool
}

// encodeAudio builds the UDPTunnel payload for an outgoing packet.
func encodeAudio(p AudioPacket) ([]byte, error) {
	m := &udp.Audio{
		Header:       &udp.Audio_Target{Target: p.Target},
		FrameNumber:  p.FrameNumber,
		OpusData:     p.Opus,
		IsTerminator: p.Terminator,
	}
	out := []byte{udpTypeAudio}
	return proto.MarshalOptions{}.MarshalAppend(out, m)
}

// decodeAudio parses a UDPTunnel payload received from the server.
func decodeAudio(b []byte) (AudioPacket, error) {
	if len(b) == 0 {
		return AudioPacket{}, errors.New("mumble: empty UDP packet")
	}
	if b[0] != udpTypeAudio {
		if b[0] == udpTypePing {
			return AudioPacket{}, fmt.Errorf("mumble: unexpected UDP ping over tunnel")
		}
		return AudioPacket{}, ErrLegacyAudio
	}
	var m udp.Audio
	if err := proto.Unmarshal(b[1:], &m); err != nil {
		return AudioPacket{}, fmt.Errorf("mumble: decode audio: %w", err)
	}
	p := AudioPacket{
		Sender:           m.GetSenderSession(),
		FrameNumber:      m.GetFrameNumber(),
		Opus:             m.GetOpusData(),
		VolumeAdjustment: m.GetVolumeAdjustment(),
		Terminator:       m.GetIsTerminator(),
	}
	switch h := m.Header.(type) {
	case *udp.Audio_Target:
		p.Target = h.Target
	case *udp.Audio_Context:
		p.Context = h.Context
	}
	return p, nil
}
