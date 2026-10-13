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
		case MsgCryptSetup, MsgPermissionQuery, MsgSuggestConfig:
			// Expected and intentionally unused (UDP crypto, ACL hints, client suggestions).
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
		if ch, ok := c.state.applyChannel(m); ok {
			ev = ChannelEvent{Channel: ch}
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
		ev = DeniedEvent{
			Type:       m.GetType(),
			Reason:     m.GetReason(),
			Permission: m.GetPermission(),
			ChannelID:  m.GetChannelId(),
			Session:    m.GetSession(),
			Name:       m.GetName(),
		}
	case *pb.Reject:
		c.cancel(&RejectError{Type: m.GetType(), Reason: m.GetReason()})
	case *pb.ServerSync:
		c.log.Warn("unexpected ServerSync after handshake")
	}
	if emit && ev != nil {
		select {
		case c.events <- ev:
		case <-c.ctx.Done():
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
