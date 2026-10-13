// Package bridge maps one browser WebSocket session to one Mumble client connection.
package bridge

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"

	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
)

// Size limits for incoming WebSocket messages.
const (
	maxJSONSize  = 16 << 10
	maxAudioSize = 4 << 10
)

// Binary frame layout (little endian), see the plan section 6.3.
const (
	frameTypeAudio    byte = 0x01
	flagTerminator    byte = 0x01
	inAudioHeaderLen       = 10 // type, flags, u64 frame_number
	outAudioHeaderLen      = 18 // type, flags, u32 sender, u64 frame_number, f32 volume
)

// --- browser -> gateway ---

// clientMsg is the union of all JSON messages a browser may send; T selects the kind.
type clientMsg struct {
	T string `json:"t"`

	// hello
	Username string    `json:"username"`
	Password string    `json:"password"`
	Cert     *certPair `json:"cert"`

	// joinChannel
	ChannelID *uint32 `json:"channelId"`

	// setSelf
	Mute *bool `json:"mute"`
	Deaf *bool `json:"deaf"`

	// text
	Message string `json:"message"`

	// ping
	TS *float64 `json:"ts"`
}

type certPair struct {
	PEM string `json:"pem"`
	Key string `json:"key"`
}

// --- gateway -> browser ---

type certMsg struct {
	T   string `json:"t"` // "cert"
	PEM string `json:"pem"`
	Key string `json:"key"`
}

type readyMsg struct {
	T            string `json:"t"` // "ready"
	Session      uint32 `json:"session"`
	Welcome      string `json:"welcome"`
	MaxBandwidth uint32 `json:"maxBandwidth"`
	AllowHTML    bool   `json:"allowHtml"`
	MessageLen   uint32 `json:"messageLength"`
}

type channelMsg struct {
	T           string   `json:"t"` // "channel"
	ID          uint32   `json:"id"`
	Parent      *uint32  `json:"parent"` // null for the root channel
	Name        string   `json:"name"`
	Position    int32    `json:"position"`
	Description string   `json:"description"`
	Temporary   bool     `json:"temporary"`
	Links       []uint32 `json:"links"`
}

type channelRemoveMsg struct {
	T  string `json:"t"` // "channelRemove"
	ID uint32 `json:"id"`
}

type userMsg struct {
	T               string `json:"t"` // "user"
	Session         uint32 `json:"session"`
	Name            string `json:"name"`
	ChannelID       uint32 `json:"channelId"`
	Mute            bool   `json:"mute"`
	Deaf            bool   `json:"deaf"`
	Suppress        bool   `json:"suppress"`
	SelfMute        bool   `json:"selfMute"`
	SelfDeaf        bool   `json:"selfDeaf"`
	PrioritySpeaker bool   `json:"prioritySpeaker"`
	Recording       bool   `json:"recording"`
	Registered      bool   `json:"registered"`
	Comment         string `json:"comment,omitempty"`
}

type userRemoveMsg struct {
	T       string `json:"t"` // "userRemove"
	Session uint32 `json:"session"`
	Actor   uint32 `json:"actor,omitempty"`
	Reason  string `json:"reason"`
	Ban     bool   `json:"ban,omitempty"`
}

type textMsg struct {
	T          string   `json:"t"` // "text"
	From       uint32   `json:"from"`
	ChannelIDs []uint32 `json:"channelIds"`
	Private    bool     `json:"private,omitempty"` // sent directly to our session
	Message    string   `json:"message"`
}

type deniedMsg struct {
	T      string `json:"t"` // "denied"
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type rejectMsg struct {
	T       string `json:"t"` // "reject"
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type pongMsg struct {
	T  string  `json:"t"` // "pong"
	TS float64 `json:"ts"`
}

type errorMsg struct {
	T       string `json:"t"` // "error"
	Message string `json:"message"`
}

func newChannelMsg(c mumble.Channel) channelMsg {
	m := channelMsg{
		T: "channel", ID: c.ID, Name: c.Name, Position: c.Position,
		Description: c.Description, Temporary: c.Temporary, Links: c.Links,
	}
	if m.Links == nil {
		m.Links = []uint32{}
	}
	if c.ID != 0 {
		p := c.Parent
		m.Parent = &p
	}
	return m
}

func newUserMsg(u mumble.User) userMsg {
	return userMsg{
		T: "user", Session: u.Session, Name: u.Name, ChannelID: u.ChannelID,
		Mute: u.Mute, Deaf: u.Deaf, Suppress: u.Suppress,
		SelfMute: u.SelfMute, SelfDeaf: u.SelfDeaf,
		PrioritySpeaker: u.PrioritySpeaker, Recording: u.Recording,
		Registered: u.Registered, Comment: u.Comment,
	}
}

func newTextMsg(e mumble.TextEvent) textMsg {
	// Tree messages are shown in the channel they were sent to, like channel messages.
	ids := slices.Concat(e.Channels, e.Trees)
	if ids == nil {
		ids = []uint32{}
	}
	return textMsg{T: "text", From: e.Actor, ChannelIDs: ids, Private: len(e.Sessions) > 0, Message: e.Message}
}

// --- binary audio frames ---

var errBadFrame = errors.New("bridge: malformed audio frame")

// parseAudioFrame decodes a browser -> gateway audio frame.
func parseAudioFrame(b []byte) (mumble.AudioPacket, error) {
	if len(b) < inAudioHeaderLen || len(b) > maxAudioSize || b[0] != frameTypeAudio {
		return mumble.AudioPacket{}, errBadFrame
	}
	p := mumble.AudioPacket{
		Terminator:  b[1]&flagTerminator != 0,
		FrameNumber: binary.LittleEndian.Uint64(b[2:10]),
		Opus:        b[inAudioHeaderLen:],
	}
	// Only a terminator may come without payload.
	if len(p.Opus) == 0 && !p.Terminator {
		return mumble.AudioPacket{}, errBadFrame
	}
	return p, nil
}

// encodeAudioFrame builds a gateway -> browser audio frame.
func encodeAudioFrame(p mumble.AudioPacket) []byte {
	b := make([]byte, outAudioHeaderLen+len(p.Opus))
	b[0] = frameTypeAudio
	if p.Terminator {
		b[1] = flagTerminator
	}
	binary.LittleEndian.PutUint32(b[2:6], p.Sender)
	binary.LittleEndian.PutUint64(b[6:14], p.FrameNumber)
	binary.LittleEndian.PutUint32(b[14:18], math.Float32bits(p.VolumeAdjustment))
	copy(b[outAudioHeaderLen:], p.Opus)
	return b
}
