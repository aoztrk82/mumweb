package mumble

import (
	"bytes"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	udp "github.com/aoztrk82/mumweb/gateway/proto/mumbleudp"
)

func TestEncodeAudio(t *testing.T) {
	b, err := encodeAudio(AudioPacket{FrameNumber: 42, Opus: []byte{1, 2, 3}, Terminator: true})
	if err != nil {
		t.Fatal(err)
	}
	if b[0] != udpTypeAudio {
		t.Fatalf("header %#x", b[0])
	}
	var m udp.Audio
	if err := proto.Unmarshal(b[1:], &m); err != nil {
		t.Fatal(err)
	}
	// target=0 must still be present on the wire (oneof set explicitly).
	if _, ok := m.Header.(*udp.Audio_Target); !ok {
		t.Fatalf("target not set: %T", m.Header)
	}
	if m.FrameNumber != 42 || !bytes.Equal(m.OpusData, []byte{1, 2, 3}) || !m.IsTerminator {
		t.Fatalf("got %v", &m)
	}
}

func TestDecodeAudio(t *testing.T) {
	m := &udp.Audio{
		Header:           &udp.Audio_Context{Context: 0},
		SenderSession:    7,
		FrameNumber:      1000,
		OpusData:         []byte{0xf8, 0xff, 0xfe},
		VolumeAdjustment: 0.5,
	}
	raw, _ := proto.Marshal(m)
	p, err := decodeAudio(append([]byte{udpTypeAudio}, raw...))
	if err != nil {
		t.Fatal(err)
	}
	if p.Sender != 7 || p.FrameNumber != 1000 || p.VolumeAdjustment != 0.5 || p.Terminator || len(p.Opus) != 3 {
		t.Fatalf("got %+v", p)
	}

	if _, err := decodeAudio([]byte{0x80, 1, 2}); !errors.Is(err, ErrLegacyAudio) {
		t.Fatalf("legacy: %v", err)
	}
	if _, err := decodeAudio(nil); err == nil {
		t.Fatal("empty accepted")
	}
}
