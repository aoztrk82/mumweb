package bridge

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
)

func TestParseAudioFrame(t *testing.T) {
	b := []byte{0x01, 0x01, 0x2a, 0, 0, 0, 0, 0, 0, 0x01, 0xf8, 0xff, 0xfe}
	p, err := parseAudioFrame(b)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Terminator || p.FrameNumber != 0x010000000000002a || !bytes.Equal(p.Opus, []byte{0xf8, 0xff, 0xfe}) {
		t.Fatalf("got %+v", p)
	}

	// Terminator without payload is fine; a non-terminator without payload is not.
	if _, err := parseAudioFrame(b[:10]); err != nil {
		t.Fatalf("empty terminator: %v", err)
	}
	noTerm := append([]byte{0x01, 0x00}, b[2:10]...)
	for name, bad := range map[string][]byte{
		"empty":      nil,
		"short":      b[:5],
		"wrong type": append([]byte{0x02}, b[1:]...),
		"no payload": noTerm,
		"too big":    append(append([]byte{}, b[:10]...), make([]byte, maxAudioSize)...),
	} {
		if _, err := parseAudioFrame(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestEncodeAudioFrame(t *testing.T) {
	got := encodeAudioFrame(mumble.AudioPacket{
		Sender: 7, FrameNumber: 258, VolumeAdjustment: 1.0, Terminator: true, Opus: []byte{9, 8},
	})
	want := []byte{
		0x01, 0x01,
		7, 0, 0, 0,
		2, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0x80, 0x3f, // float32(1.0) LE
		9, 8,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

func TestChannelMsgRootHasNullParent(t *testing.T) {
	root, _ := json.Marshal(newChannelMsg(mumble.Channel{ID: 0, Name: "Root"}))
	sub, _ := json.Marshal(newChannelMsg(mumble.Channel{ID: 3, Parent: 0, Name: "Oyun"}))
	if !bytes.Contains(root, []byte(`"parent":null`)) || !bytes.Contains(sub, []byte(`"parent":0`)) {
		t.Fatalf("root=%s sub=%s", root, sub)
	}
	if !bytes.Contains(root, []byte(`"links":[]`)) {
		t.Fatalf("links should be an empty array: %s", root)
	}
}

func TestTokenBucket(t *testing.T) {
	var b tokenBucket
	now := time.Unix(0, 0)
	for i := range 3 {
		if !b.take(now, 1, 3) {
			t.Fatalf("take %d denied", i)
		}
	}
	if b.take(now, 1, 3) {
		t.Fatal("burst exceeded")
	}
	if !b.take(now.Add(time.Second), 1, 3) {
		t.Fatal("not refilled")
	}
}

// parseIncomingForTest decodes a gateway -> browser audio frame.
func parseIncomingForTest(b []byte) (mumble.AudioPacket, error) {
	if len(b) < outAudioHeaderLen || b[0] != frameTypeAudio {
		return mumble.AudioPacket{}, errBadFrame
	}
	return mumble.AudioPacket{
		Terminator:  b[1]&flagTerminator != 0,
		Sender:      uint32(b[2]) | uint32(b[3])<<8 | uint32(b[4])<<16 | uint32(b[5])<<24,
		FrameNumber: uint64(b[6]) | uint64(b[7])<<8 | uint64(b[8])<<16 | uint64(b[9])<<24,
		Opus:        b[outAudioHeaderLen:],
	}, nil
}

func TestNormalizeOrigin(t *testing.T) {
	for in, want := range map[string]string{
		"https://ses.example.com":      "https://ses.example.com",
		"https://Ses.Example.com:443/": "https://ses.example.com",
		"https://mumweb.test:8443":     "https://mumweb.test:8443",
		"http://localhost:80":          "http://localhost",
		"http://localhost:5173":        "http://localhost:5173",
		"https://[::1]:443":            "https://[::1]",
		" https://ses.example.com/ ":   "https://ses.example.com",
		"null":                         "null",
	} {
		if got := normalizeOrigin(in); got != want {
			t.Errorf("normalizeOrigin(%q) = %q, want %q", in, got, want)
		}
	}
}
