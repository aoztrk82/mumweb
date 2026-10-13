package mumble

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

// Reported client version. 1.5.0 or newer makes the server send protobuf audio.
const (
	versionMajor = 1
	versionMinor = 5
	versionPatch = 0
)

const (
	pingInterval     = 15 * time.Second
	readTimeout      = 60 * time.Second // server answers pings, so this only trips on a dead link
	writeTimeout     = 10 * time.Second
	handshakeTimeout = 20 * time.Second
	eventBuffer      = 256
)

// Config describes how to connect and authenticate.
type Config struct {
	Addr     string // host:port
	Username string
	Password string
	Tokens   []string
	// Certificate is the optional client certificate (identity) presented to the server.
	Certificate *tls.Certificate
	// TLSVerify enables normal verification of the server certificate chain and name.
	TLSVerify bool
	// Fingerprint, if set, pins the server leaf certificate by SHA-256 (hex, ':' optional).
	Fingerprint string
	// Release is the client name shown to other users ("mumweb x.y").
	Release string
	OS      string
	Logger  *slog.Logger
}

// RejectError is returned by Dial when the server refuses the login.
type RejectError struct {
	Type   pb.Reject_RejectType
	Reason string
}

func (e *RejectError) Error() string {
	return fmt.Sprintf("mumble: rejected (%s): %s", e.Type, e.Reason)
}

// ServerInfo is what the server tells us during and right after the handshake.
type ServerInfo struct {
	Session        uint32
	MaxBandwidth   uint32
	WelcomeText    string
	Permissions    uint64
	ServerVersion  uint64 // version_v2 encoding
	ServerRelease  string
	AllowHTML      bool
	MessageLength  uint32
	ImageMsgLength uint32
	MaxUsers       uint32
}

// Client is one authenticated connection to a Mumble server.
type Client struct {
	conn   net.Conn
	log    *slog.Logger
	state  *State
	events chan Event

	infoMu sync.Mutex
	info   ServerInfo

	writeMu sync.Mutex

	banMu       sync.Mutex
	pendingBans []pendingBan

	ctx    context.Context
	cancel context.CancelCauseFunc
	wg     sync.WaitGroup
}

// Dial connects, authenticates and waits for ServerSync. On success the initial
// channel/user state is available via State(); subsequent changes arrive on Events().
func Dial(ctx context.Context, cfg Config) (*Client, error) {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	tlsCfg, err := tlsConfig(cfg)
	if err != nil {
		return nil, err
	}

	hsCtx, hsCancel := context.WithTimeout(ctx, handshakeTimeout)
	defer hsCancel()

	d := tls.Dialer{NetDialer: &net.Dialer{KeepAlive: 30 * time.Second}, Config: tlsCfg}
	raw, err := d.DialContext(hsCtx, "tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("mumble: dial %s: %w", cfg.Addr, err)
	}
	conn := raw.(*tls.Conn)

	cctx, cancel := context.WithCancelCause(context.Background())
	c := &Client{
		conn:   conn,
		log:    log,
		state:  newState(),
		events: make(chan Event, eventBuffer),
		ctx:    cctx,
		cancel: cancel,
	}

	// Abort the blocking handshake reads if ctx/timeout fires.
	stop := context.AfterFunc(hsCtx, func() { _ = conn.SetDeadline(time.Now()) })
	r := bufio.NewReader(conn)
	err = c.handshake(r, cfg)
	if !stop() || err != nil {
		_ = conn.Close()
		cancel(err)
		if err == nil || hsCtx.Err() != nil {
			err = fmt.Errorf("mumble: handshake: %w", context.Cause(hsCtx))
		}
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})

	c.wg.Add(2)
	go c.readLoop(r)
	go c.pingLoop()
	go func() {
		<-cctx.Done()
		_ = conn.Close()
	}()
	return c, nil
}

func tlsConfig(cfg Config) (*tls.Config, error) {
	tc := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.Certificate != nil {
		tc.Certificates = []tls.Certificate{*cfg.Certificate}
	}
	if cfg.TLSVerify {
		host, _, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			return nil, fmt.Errorf("mumble: bad address %q: %w", cfg.Addr, err)
		}
		tc.ServerName = host
	} else {
		// Mumble servers usually use self-signed certificates; the gateway only
		// talks to a fixed server on an internal network (optionally pinned below).
		tc.InsecureSkipVerify = true
	}
	if cfg.Fingerprint != "" {
		want, err := hex.DecodeString(strings.ReplaceAll(cfg.Fingerprint, ":", ""))
		if err != nil || len(want) != sha256.Size {
			return nil, fmt.Errorf("mumble: invalid SHA-256 fingerprint %q", cfg.Fingerprint)
		}
		tc.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("mumble: server sent no certificate")
			}
			got := sha256.Sum256(rawCerts[0])
			if subtle.ConstantTimeCompare(got[:], want) != 1 {
				return fmt.Errorf("mumble: server certificate fingerprint mismatch (got %x)", got)
			}
			return nil
		}
	}
	return tc, nil
}

func (c *Client) handshake(r *bufio.Reader, cfg Config) error {
	release, osName := cfg.Release, cfg.OS
	if release == "" {
		release = "mumweb"
	}
	if osName == "" {
		osName = "Web"
	}
	v1 := uint32(versionMajor<<16 | versionMinor<<8 | versionPatch)
	v2 := uint64(versionMajor)<<48 | uint64(versionMinor)<<32 | uint64(versionPatch)<<16
	if err := c.send(&pb.Version{
		VersionV1: &v1,
		VersionV2: &v2,
		Release:   &release,
		Os:        &osName,
	}); err != nil {
		return err
	}
	auth := &pb.Authenticate{
		Username: proto.String(cfg.Username),
		Tokens:   cfg.Tokens,
		Opus:     proto.Bool(true),
	}
	if cfg.Password != "" {
		auth.Password = proto.String(cfg.Password)
	}
	if err := c.send(auth); err != nil {
		return err
	}

	// The server sends ServerConfig right after ServerSync; wait for both so
	// Info() is complete when Dial returns.
	synced := false
	for {
		t, payload, err := readFrame(r)
		if err != nil {
			return fmt.Errorf("mumble: handshake read: %w", err)
		}
		msg, err := c.decode(t, payload)
		if err != nil {
			return err
		}
		switch m := msg.(type) {
		case *pb.Reject:
			return &RejectError{Type: m.GetType(), Reason: m.GetReason()}
		case *pb.ServerSync:
			c.info.Session = m.GetSession()
			c.info.MaxBandwidth = m.GetMaxBandwidth()
			c.info.WelcomeText = m.GetWelcomeText()
			c.info.Permissions = m.GetPermissions()
			synced = true
		default:
			// No events are emitted during the handshake; the caller reads State().
			c.handle(t, msg, payload, false)
			if t == MsgServerConfig && synced {
				return nil
			}
		}
	}
}

// decode unmarshals a frame payload. It returns (nil, nil) for ignored types.
func (c *Client) decode(t MsgType, payload []byte) (proto.Message, error) {
	msg := newMessage(t)
	if msg == nil {
		return nil, nil
	}
	if err := proto.Unmarshal(payload, msg); err != nil {
		return nil, fmt.Errorf("mumble: decode %s: %w", t, err)
	}
	return msg, nil
}

// handle applies a message to the client state and, if emit is set, publishes an event.
func (c *Client) handle(t MsgType, msg proto.Message, payload []byte, emit bool) {
	var ev Event
	var extra []Event // additional events caused by the same message
	switch m := msg.(type) {
	case nil:
		switch t {
		case MsgUDPTunnel:
			p, err := decodeAudio(payload)
			if err != nil {
				c.log.Debug("dropping tunnelled UDP packet", "err", err, "len", len(payload))
				return
			}
			ev = AudioEvent{Packet: p}
		case MsgCryptSetup, MsgSuggestConfig:
			// Expected and intentionally unused (UDP crypto, client suggestions).
		default:
			c.log.Debug("ignoring message", "type", t.String(), "len", len(payload))
		}
	case *pb.Version:
		c.info.ServerVersion = m.GetVersionV2()
		if c.info.ServerVersion == 0 && m.VersionV1 != nil {
			v := uint64(m.GetVersionV1())
			c.info.ServerVersion = (v>>16)<<48 | ((v>>8)&0xff)<<32 | (v&0xff)<<16
		}
		c.info.ServerRelease = m.GetRelease()
	case *pb.Ping:
		// Reply to our own ping; nothing to do.
	case *pb.CodecVersion:
		if !m.GetOpus() {
			c.log.Warn("server does not advertise Opus support")
		}
	case *pb.ServerConfig:
		c.infoMu.Lock()
		if m.MaxBandwidth != nil {
			c.info.MaxBandwidth = m.GetMaxBandwidth()
		}
		if m.WelcomeText != nil {
			c.info.WelcomeText = m.GetWelcomeText()
		}
		c.info.AllowHTML = m.GetAllowHtml()
		c.info.MessageLength = m.GetMessageLength()
		c.info.ImageMsgLength = m.GetImageMessageLength()
		c.info.MaxUsers = m.GetMaxUsers()
		ev = ServerConfigEvent{Info: c.info}
		c.infoMu.Unlock()
	case *pb.ChannelState:
		if ch, others, ok := c.state.applyChannel(m); ok {
			ev = ChannelEvent{Channel: ch}
			for _, o := range others {
				extra = append(extra, ChannelEvent{Channel: o})
			}
		}
	case *pb.ChannelRemove:
		c.state.removeChannel(m.GetChannelId())
		ev = ChannelRemoveEvent{ID: m.GetChannelId()}
	case *pb.UserState:
		if u, ok := c.state.applyUser(m); ok {
			ev = UserEvent{User: u, Actor: m.GetActor()}
		}
	case *pb.UserRemove:
		c.state.removeUser(m.GetSession())
		ev = UserRemoveEvent{Session: m.GetSession(), Actor: m.GetActor(), Reason: m.GetReason(), Ban: m.GetBan()}
	case *pb.TextMessage:
		ev = TextEvent{Actor: m.GetActor(), Sessions: m.Session, Channels: m.ChannelId, Trees: m.TreeId, Message: m.GetMessage()}
	case *pb.PermissionDenied:
		if m.GetPermission()&PermBan != 0 {
			c.dropPendingBans()
		}
		ev = DeniedEvent{
			Type:       m.GetType(),
			Reason:     m.GetReason(),
			Permission: m.GetPermission(),
			ChannelID:  m.GetChannelId(),
			Session:    m.GetSession(),
			Name:       m.GetName(),
		}
	case *pb.PermissionQuery:
		ev = PermissionEvent{ChannelID: m.GetChannelId(), Permissions: m.GetPermissions(), Flush: m.GetFlush()}
	case *pb.UserStats:
		ev = UserStatsEvent{Stats: m}
	case *pb.BanList:
		c.completeBans(m)
	case *pb.Reject:
		c.cancel(&RejectError{Type: m.GetType(), Reason: m.GetReason()})
	case *pb.ServerSync:
		c.log.Warn("unexpected ServerSync after handshake")
	}
	if !emit || ev == nil {
		return
	}
	for _, e := range append([]Event{ev}, extra...) {
		select {
		case c.events <- e:
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) readLoop(r *bufio.Reader) {
	defer c.wg.Done()
	defer close(c.events)
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		t, payload, err := readFrame(r)
		if err != nil {
			if c.ctx.Err() == nil {
				c.cancel(fmt.Errorf("mumble: read: %w", err))
			}
			return
		}
		msg, err := c.decode(t, payload)
		if err != nil {
			// A single malformed message is logged and skipped.
			c.log.Warn("bad message from server", "type", t.String(), "err", err)
			continue
		}
		c.handle(t, msg, payload, true)
	}
}

func (c *Client) pingLoop() {
	defer c.wg.Done()
	tick := time.NewTicker(pingInterval)
	defer tick.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case now := <-tick.C:
			ts := uint64(now.UnixMilli())
			if err := c.send(&pb.Ping{Timestamp: &ts}); err != nil {
				c.cancel(fmt.Errorf("mumble: ping: %w", err))
				return
			}
		}
	}
}

func (c *Client) send(m proto.Message) error {
	t, ok := msgTypeOf(m)
	if !ok {
		return fmt.Errorf("mumble: unsupported outgoing message %T", m)
	}
	payload, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	return c.writeRaw(t, payload)
}

func (c *Client) writeRaw(t MsgType, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := writeFrame(c.conn, t, payload); err != nil {
		if c.ctx.Err() == nil {
			c.cancel(fmt.Errorf("mumble: write: %w", err))
		}
		return err
	}
	return nil
}

// Events returns the event stream. It is closed when the connection ends.
// The consumer must keep reading; a stalled consumer stalls the connection.
func (c *Client) Events() <-chan Event { return c.events }

// State returns the live channel/user state.
func (c *Client) State() *State { return c.state }

// Info returns server information gathered during the handshake.
// Fields may be updated by ServerConfig; use the ServerConfigEvent for changes.
func (c *Client) Info() ServerInfo {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()
	return c.info
}

// ServerFingerprint returns the SHA-256 fingerprint of the server's TLS certificate
// in the colon-separated hex form accepted by Config.Fingerprint.
func (c *Client) ServerFingerprint() string {
	tc, ok := c.conn.(*tls.Conn)
	if !ok {
		return ""
	}
	certs := tc.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return ""
	}
	sum := sha256.Sum256(certs[0].Raw)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

// Session returns our own session ID (fixed after Dial).
func (c *Client) Session() uint32 { return c.info.Session }

// Done is closed when the connection has ended.
func (c *Client) Done() <-chan struct{} { return c.ctx.Done() }

// Err returns why the connection ended (nil while it is open).
func (c *Client) Err() error {
	if c.ctx.Err() == nil {
		return nil
	}
	return context.Cause(c.ctx)
}

// Close terminates the connection and waits for the internal goroutines.
// Events still buffered remain readable until the channel is drained.
func (c *Client) Close() error {
	c.cancel(ErrClosed)
	c.wg.Wait()
	return nil
}

// ErrClosed is the cause recorded when Close is called.
var ErrClosed = errors.New("mumble: closed")

// SendAudio sends one voice packet over the TCP tunnel.
func (c *Client) SendAudio(p AudioPacket) error {
	payload, err := encodeAudio(p)
	if err != nil {
		return err
	}
	return c.writeRaw(MsgUDPTunnel, payload)
}

// JoinChannel moves ourselves to another channel.
func (c *Client) JoinChannel(id uint32) error {
	sess := c.Session()
	return c.send(&pb.UserState{Session: &sess, ChannelId: &id})
}

// SetSelf updates our self-mute/self-deaf flags. Deafening implies muting (server rule).
func (c *Client) SetSelf(mute, deaf bool) error {
	sess := c.Session()
	if deaf {
		mute = true
	}
	return c.send(&pb.UserState{Session: &sess, SelfMute: &mute, SelfDeaf: &deaf})
}

// SendText sends an (HTML) text message to the given channels and/or sessions.
func (c *Client) SendText(message string, channels, sessions []uint32) error {
	return c.send(&pb.TextMessage{Message: &message, ChannelId: channels, Session: sessions})
}

// SendTreeText sends a text message to a channel and all its sub-channels.
func (c *Client) SendTreeText(message string, channel uint32) error {
	return c.send(&pb.TextMessage{Message: &message, TreeId: []uint32{channel}})
}

// ChannelEdit holds the channel properties to change; nil fields are left as they are.
type ChannelEdit struct {
	Name        *string
	Description *string
	Position    *int32
	MaxUsers    *uint32
}

// EditChannel changes channel properties (needs Write on the channel).
func (c *Client) EditChannel(id uint32, e ChannelEdit) error {
	return c.send(&pb.ChannelState{ChannelId: &id, Name: e.Name, Description: e.Description, Position: e.Position, MaxUsers: e.MaxUsers})
}

// RemoveChannel deletes a channel and its sub-channels (needs Write).
func (c *Client) RemoveChannel(id uint32) error {
	return c.send(&pb.ChannelRemove{ChannelId: &id})
}

// LinkChannels adds/removes links of a channel (needs LinkChannel on both ends).
func (c *Client) LinkChannels(id uint32, add, remove []uint32) error {
	return c.send(&pb.ChannelState{ChannelId: &id, LinksAdd: add, LinksRemove: remove})
}

// SetListening starts/stops listening to channels without being in them (needs Listen).
func (c *Client) SetListening(add, remove []uint32) error {
	sess := c.Session()
	return c.send(&pb.UserState{Session: &sess, ListeningChannelAdd: add, ListeningChannelRemove: remove})
}

// RequestChannelDescription asks for a channel's full description (sent as ChannelState).
func (c *Client) RequestChannelDescription(id uint32) error {
	return c.send(&pb.RequestBlob{ChannelDescription: []uint32{id}})
}

// CreateChannel asks the server to create a sub-channel. Permanent channels need the
// MakeChannel permission, temporary ones (removed when empty) MakeTempChannel.
// The server answers with ChannelState on success or PermissionDenied.
func (c *Client) CreateChannel(parent uint32, name string, temporary bool) error {
	m := &pb.ChannelState{Parent: &parent, Name: &name}
	if temporary {
		m.Temporary = &temporary
	}
	return c.send(m)
}

// ACL permission bits (Mumble src/ACL.h).
const (
	PermWrite            = 0x1
	PermTraverse         = 0x2
	PermEnter            = 0x4
	PermSpeak            = 0x8
	PermMuteDeafen       = 0x10
	PermMove             = 0x20
	PermMakeChannel      = 0x40
	PermLinkChannel      = 0x80
	PermWhisper          = 0x100
	PermTextMessage      = 0x200
	PermMakeTempChannel  = 0x400
	PermListen           = 0x800
	PermKick             = 0x10000
	PermBan              = 0x20000
	PermRegister         = 0x40000
	PermSelfRegister     = 0x80000
	PermResetUserContent = 0x100000
)

// QueryPermissions asks for our permissions in a channel; the answer arrives as PermissionEvent.
func (c *Client) QueryPermissions(channelID uint32) error {
	return c.send(&pb.PermissionQuery{ChannelId: &channelID})
}

// UpdateUser sends an administrative UserState change for another (or our own) session.
// Only the non-nil fields are sent; the server checks permissions.
type UserUpdate struct {
	ChannelID       *uint32
	Mute            *bool
	Deaf            *bool
	PrioritySpeaker *bool
	ResetComment    bool
	ResetTexture    bool
	Register        bool
}

func (c *Client) UpdateUser(session uint32, u UserUpdate) error {
	m := &pb.UserState{Session: &session, ChannelId: u.ChannelID, Mute: u.Mute, Deaf: u.Deaf, PrioritySpeaker: u.PrioritySpeaker}
	if u.ResetComment {
		m.Comment = proto.String("")
	}
	if u.ResetTexture {
		m.Texture = []byte{}
	}
	if u.Register {
		m.UserId = proto.Uint32(0) // the server assigns the real id
	}
	return c.send(m)
}

// SetComment sets our own comment (HTML).
func (c *Client) SetComment(comment string) error {
	sess := c.Session()
	return c.send(&pb.UserState{Session: &sess, Comment: &comment})
}

// Kick removes a user from the server (needs Kick or Ban on the root channel).
func (c *Client) Kick(session uint32, reason string) error {
	return c.send(&pb.UserRemove{Session: &session, Reason: &reason})
}

// RequestComment asks for a user's full comment (sent as UserState).
func (c *Client) RequestComment(session uint32) error {
	return c.send(&pb.RequestBlob{SessionComment: []uint32{session}})
}

// RequestUserStats asks for statistics about a user (answer: UserStatsEvent).
func (c *Client) RequestUserStats(session uint32) error {
	return c.send(&pb.UserStats{Session: &session, StatsOnly: proto.Bool(false)})
}

// banAddress is 100:: (RFC 6666 discard-only prefix). A ban entry needs a non-zero
// address; with a /128 mask this one never matches a real peer, so the entry
// effectively bans by certificate hash only.
var banAddress = append([]byte{0x01, 0x00}, make([]byte, 14)...)

type pendingBan struct {
	session uint32
	name    string
	hash    string
	reason  string
}

// BanByCertificate bans a user by certificate hash only, then kicks them.
//
// Mumble's own kick-ban (UserRemove with ban=true) also bans the user's IP address.
// All web users reach the server from the gateway's address, so that would lock out
// every web user. Instead the ban list is fetched, extended with a hash-only entry and
// written back (needs the Ban permission on the root channel).
func (c *Client) BanByCertificate(session uint32, reason string) error {
	u, ok := c.state.User(session)
	if !ok {
		return fmt.Errorf("mumble: unknown session %d", session)
	}
	if u.Hash == "" {
		return errors.New("mumble: user has no certificate to ban")
	}
	c.banMu.Lock()
	c.pendingBans = append(c.pendingBans, pendingBan{session: session, name: u.Name, hash: u.Hash, reason: reason})
	c.banMu.Unlock()
	return c.send(&pb.BanList{Query: proto.Bool(true)})
}

// completeBans handles the BanList reply to our query: append pending entries, write
// the list back and kick the banned users.
func (c *Client) completeBans(list *pb.BanList) {
	c.banMu.Lock()
	pending := c.pendingBans
	c.pendingBans = nil
	c.banMu.Unlock()
	if len(pending) == 0 {
		return
	}
	bans := list.GetBans()
	for _, p := range pending {
		bans = append(bans, &pb.BanList_BanEntry{
			Address:  banAddress,
			Mask:     proto.Uint32(128),
			Name:     proto.String(p.name),
			Hash:     proto.String(p.hash),
			Reason:   proto.String(p.reason),
			Duration: proto.Uint32(0),
		})
	}
	if err := c.send(&pb.BanList{Bans: bans}); err != nil {
		return
	}
	for _, p := range pending {
		_ = c.Kick(p.session, p.reason)
	}
}

func (c *Client) dropPendingBans() {
	c.banMu.Lock()
	c.pendingBans = nil
	c.banMu.Unlock()
}
