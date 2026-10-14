// Package bridge maps one browser WebSocket session to one Mumble client connection.
package bridge

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"slices"

	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

// Size limits for incoming WebSocket messages.
const (
	maxJSONSize  = 16 << 10
	maxAudioSize = 4 << 10
	// Admin sessions may send whole ban lists and ACLs.
	maxAdminJSONSize = 512 << 10
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

	// hello (Tokens: remembered channel passwords / access tokens; also setTokens)
	Username string `json:"username"`
	Password string `json:"password"`
	// AccessPassword is the web access password (MUMWEB_ACCESS_PASSWORD), not the Mumble server password.
	AccessPassword string `json:"accessPassword"`
	// ReservedPassword opens the reserved web slots (MUMWEB_RESERVED_PASSWORD); optional.
	ReservedPassword string    `json:"reservedPassword"`
	Cert             *certPair `json:"cert"`
	Tokens           []string  `json:"tokens"`
	// Role is "admin" for the /admin panel (needs AdminPassword); anything else is a normal session.
	Role          string `json:"role"`
	AdminPassword string `json:"adminPassword"`

	// joinChannel
	ChannelID *uint32 `json:"channelId"`

	// createChannel
	Parent    *uint32 `json:"parent"`
	Name      string  `json:"name"`
	Temporary bool    `json:"temporary"`

	// user actions (moveUser, setUser, kick, ban, register, resetComment,
	// resetTexture, requestComment, userStats) and private text
	Session         *uint32 `json:"session"`
	Reason          string  `json:"reason"`
	PrioritySpeaker *bool   `json:"prioritySpeaker"`

	// channel actions (editChannel, removeChannel, linkChannels, listen, requestDescription)
	// and tree text messages
	Description *string  `json:"description"`
	NewName     *string  `json:"newName"`
	Position    *int32   `json:"position"`
	MaxUsers    *uint32  `json:"maxUsers"`
	Add         []uint32 `json:"add"`
	Remove      []uint32 `json:"remove"`
	On          *bool    `json:"on"`
	Tree        bool     `json:"tree"`
	ChannelPass *string  `json:"channelPassword"` // setChannelPassword

	// setSelf
	Mute *bool `json:"mute"`
	Deaf *bool `json:"deaf"`

	// text
	Message string `json:"message"`

	// ping
	TS *float64 `json:"ts"`

	// admin panel (admin sessions only): adminSetBanList, adminUpdateUsers, adminACL, adminSetACL
	Bans        []banEntryJSON   `json:"bans"`
	UserUpdates []userUpdateJSON `json:"userUpdates"`
	ACL         *aclJSON         `json:"acl"`
}

// --- admin panel payloads ---

type banEntryJSON struct {
	Address  string `json:"address"`
	Mask     uint32 `json:"mask"`
	Name     string `json:"name,omitempty"`
	Hash     string `json:"hash,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Start    string `json:"start,omitempty"`
	Duration uint32 `json:"duration"`
}

type userUpdateJSON struct {
	ID   uint32  `json:"id"`
	Name *string `json:"name"` // null deletes the registration
}

type registeredUserJSON struct {
	ID          uint32  `json:"id"`
	Name        string  `json:"name"`
	LastSeen    string  `json:"lastSeen,omitempty"`
	LastChannel *uint32 `json:"lastChannel,omitempty"`
}

type aclEntryJSON struct {
	ApplyHere bool    `json:"applyHere"`
	ApplySubs bool    `json:"applySubs"`
	Inherited bool    `json:"inherited"`
	UserID    *uint32 `json:"userId,omitempty"`
	Group     string  `json:"group,omitempty"`
	Grant     uint32  `json:"grant"`
	Deny      uint32  `json:"deny"`
}

type aclGroupJSON struct {
	Name             string   `json:"name"`
	Inherited        bool     `json:"inherited"`
	Inherit          bool     `json:"inherit"`
	Inheritable      bool     `json:"inheritable"`
	Add              []uint32 `json:"add"`
	Remove           []uint32 `json:"remove"`
	InheritedMembers []uint32 `json:"inheritedMembers"`
}

type aclJSON struct {
	InheritACLs bool           `json:"inheritAcls"`
	ACLs        []aclEntryJSON `json:"acls"`
	Groups      []aclGroupJSON `json:"groups"`
}

type banListMsg struct {
	T    string         `json:"t"` // "banList"
	Bans []banEntryJSON `json:"bans"`
}

type userListMsg struct {
	T     string               `json:"t"` // "userList"
	Users []registeredUserJSON `json:"users"`
}

type aclMsg struct {
	T         string `json:"t"` // "acl"
	ChannelID uint32 `json:"channelId"`
	aclJSON
}

// adminBusyMsg tells the panel that an admin command was dropped by the pacing limit.
type adminBusyMsg struct {
	T string `json:"t"` // "adminBusy"
}

func newBanListMsg(bans []mumble.BanEntry) banListMsg {
	out := banListMsg{T: "banList", Bans: make([]banEntryJSON, 0, len(bans))}
	for _, b := range bans {
		out.Bans = append(out.Bans, banEntryJSON(b))
	}
	return out
}

func newUserListMsg(users []mumble.RegisteredUser) userListMsg {
	out := userListMsg{T: "userList", Users: make([]registeredUserJSON, 0, len(users))}
	for _, u := range users {
		out.Users = append(out.Users, registeredUserJSON(u))
	}
	return out
}

func newACLMsg(a mumble.ChannelACL) aclMsg {
	m := aclMsg{T: "acl", ChannelID: a.ChannelID}
	m.InheritACLs = a.InheritACLs
	m.ACLs = make([]aclEntryJSON, 0, len(a.ACLs))
	for _, e := range a.ACLs {
		m.ACLs = append(m.ACLs, aclEntryJSON(e))
	}
	m.Groups = make([]aclGroupJSON, 0, len(a.Groups))
	for _, g := range a.Groups {
		m.Groups = append(m.Groups, aclGroupJSON{Name: g.Name, Inherited: g.Inherited, Inherit: g.Inherit, Inheritable: g.Inheritable,
			Add: nonNil(g.Add), Remove: nonNil(g.Remove), InheritedMembers: nonNil(g.InheritedMembers)})
	}
	return m
}

func nonNil(s []uint32) []uint32 {
	if s == nil {
		return []uint32{}
	}
	return s
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
	Permissions  uint64 `json:"permissions"` // ACL bits in the root channel
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
	HasDesc     bool     `json:"hasDescription"`
	Temporary   bool     `json:"temporary"`
	MaxUsers    uint32   `json:"maxUsers"`
	Links       []uint32 `json:"links"`
	Restricted  bool     `json:"restricted"` // entering needs a password/token
	CanEnter    bool     `json:"canEnter"`
}

type channelPasswordMsg struct {
	T         string `json:"t"` // "channelPassword"
	ChannelID uint32 `json:"channelId"`
	Password  string `json:"password"`
}

type channelRemoveMsg struct {
	T  string `json:"t"` // "channelRemove"
	ID uint32 `json:"id"`
}

type userMsg struct {
	T               string   `json:"t"` // "user"
	Session         uint32   `json:"session"`
	Name            string   `json:"name"`
	ChannelID       uint32   `json:"channelId"`
	Mute            bool     `json:"mute"`
	Deaf            bool     `json:"deaf"`
	Suppress        bool     `json:"suppress"`
	SelfMute        bool     `json:"selfMute"`
	SelfDeaf        bool     `json:"selfDeaf"`
	PrioritySpeaker bool     `json:"prioritySpeaker"`
	Recording       bool     `json:"recording"`
	Registered      bool     `json:"registered"`
	UserID          *uint32  `json:"userId,omitempty"` // registration id; 0 = SuperUser
	Comment         string   `json:"comment,omitempty"`
	HasComment      bool     `json:"hasComment"`
	HasTexture      bool     `json:"hasTexture"`
	Hash            string   `json:"hash,omitempty"` // certificate hash (public in Mumble)
	Listening       []uint32 `json:"listening"`      // channels this user listens to
}

type permissionsMsg struct {
	T           string `json:"t"` // "permissions"
	ChannelID   uint32 `json:"channelId"`
	Permissions uint32 `json:"permissions"`
	Flush       bool   `json:"flush"`
}

type userStatsMsg struct {
	T          string       `json:"t"` // "userStats"
	Session    uint32       `json:"session"`
	Release    string       `json:"release,omitempty"`
	OS         string       `json:"os,omitempty"`
	OSVersion  string       `json:"osVersion,omitempty"`
	Version    string       `json:"version,omitempty"`
	OnlineSecs *uint32      `json:"onlineSecs,omitempty"`
	IdleSecs   *uint32      `json:"idleSecs,omitempty"`
	Address    string       `json:"address,omitempty"`
	Bandwidth  *uint32      `json:"bandwidth,omitempty"`
	TCPPingAvg float32      `json:"tcpPingAvg"`
	UDPPingAvg float32      `json:"udpPingAvg"`
	TCPPackets uint32       `json:"tcpPackets"`
	UDPPackets uint32       `json:"udpPackets"`
	Opus       bool         `json:"opus"`
	StrongCert bool         `json:"strongCertificate"`
	CertCount  int          `json:"certificates"`
	FromClient *packetStats `json:"fromClient,omitempty"`
	FromServer *packetStats `json:"fromServer,omitempty"`
}

type packetStats struct {
	Good   uint32 `json:"good"`
	Late   uint32 `json:"late"`
	Lost   uint32 `json:"lost"`
	Resync uint32 `json:"resync"`
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
	T          string `json:"t"` // "denied"
	Type       string `json:"type"`
	Reason     string `json:"reason"`
	Permission uint32 `json:"permission,omitempty"` // ACL bit for type "Permission"
	ChannelID  uint32 `json:"channelId"`
	Session    uint32 `json:"session,omitempty"`
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
	T       string `json:"t"`    // "error"
	Code    string `json:"code"` // see errCode* constants
	Message string `json:"message"`
}

// Error codes for errorMsg.
const (
	errCodeInvalidCert       = "invalid_certificate"
	errCodeMumbleUnavailable = "mumble_unavailable"
	errCodeDisconnected      = "disconnected"
)

func newChannelMsg(c mumble.Channel) channelMsg {
	m := channelMsg{
		T: "channel", ID: c.ID, Name: c.Name, Position: c.Position,
		Description: c.Description, HasDesc: c.Description != "" || c.HasDescriptionHash,
		Temporary: c.Temporary, MaxUsers: c.MaxUsers, Links: c.Links,
		Restricted: c.EnterRestricted, CanEnter: c.CanEnter,
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
	m := userMsg{
		T: "user", Session: u.Session, Name: u.Name, ChannelID: u.ChannelID,
		Mute: u.Mute, Deaf: u.Deaf, Suppress: u.Suppress,
		SelfMute: u.SelfMute, SelfDeaf: u.SelfDeaf,
		PrioritySpeaker: u.PrioritySpeaker, Recording: u.Recording,
		Registered: u.Registered, Comment: u.Comment,
		HasComment: u.Comment != "" || u.HasCommentHash, HasTexture: u.HasTexture, Hash: u.Hash,
		Listening: u.Listening,
	}
	if m.Listening == nil {
		m.Listening = []uint32{}
	}
	if u.Registered {
		id := u.UserID
		m.UserID = &id
	}
	return m
}

func newUserStatsMsg(st *pb.UserStats) userStatsMsg {
	m := userStatsMsg{
		T: "userStats", Session: st.GetSession(),
		OnlineSecs: st.Onlinesecs, IdleSecs: st.Idlesecs, Bandwidth: st.Bandwidth,
		TCPPingAvg: st.GetTcpPingAvg(), UDPPingAvg: st.GetUdpPingAvg(),
		TCPPackets: st.GetTcpPackets(), UDPPackets: st.GetUdpPackets(),
		Opus: st.GetOpus(), StrongCert: st.GetStrongCertificate(), CertCount: len(st.Certificates),
	}
	if v := st.GetVersion(); v != nil {
		m.Release, m.OS, m.OSVersion = v.GetRelease(), v.GetOs(), v.GetOsVersion()
		if v2 := v.GetVersionV2(); v2 != 0 {
			m.Version = fmt.Sprintf("%d.%d.%d", v2>>48, (v2>>32)&0xffff, (v2>>16)&0xffff)
		} else if v1 := v.GetVersionV1(); v1 != 0 {
			m.Version = fmt.Sprintf("%d.%d.%d", v1>>16, (v1>>8)&0xff, v1&0xff)
		}
	}
	if a := st.GetAddress(); len(a) == 16 {
		ip := net.IP(a)
		if v4 := ip.To4(); v4 != nil {
			ip = v4
		}
		m.Address = ip.String()
	}
	conv := func(s *pb.UserStats_Stats) *packetStats {
		if s == nil {
			return nil
		}
		return &packetStats{Good: s.GetGood(), Late: s.GetLate(), Lost: s.GetLost(), Resync: s.GetResync()}
	}
	m.FromClient, m.FromServer = conv(st.FromClient), conv(st.FromServer)
	return m
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
