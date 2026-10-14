package bridge

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"

	"github.com/aoztrk82/mumweb/gateway/internal/certs"
	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
)

const (
	helloTimeout      = 10 * time.Second
	browserIdle       = 60 * time.Second // browsers ping every 20 s
	wsWriteTimeout    = 10 * time.Second
	maxUsernameLen    = 128
	maxChannelNameLen = 128
	maxReasonLen      = 512
	maxTokens         = 32
	maxTokenLen       = 128
	audioPacketsPerS  = 100
	audioPacketsBurst = 150
	// JSON control messages (joins, permission queries, chat...). Generous, since a client
	// asks for the permissions of every channel after login; it only stops floods.
	controlPerS       = 50
	controlBurst      = 200
	controlFloodLimit = 500 // consecutive dropped messages that end the session
)

// Config configures the WebSocket bridge.
type Config struct {
	MumbleAddr     string
	TLSVerify      bool
	Fingerprint    string
	MaxSessions    int
	MaxPerIP       int      // concurrent sessions per client IP; 0 disables the limit
	AllowedOrigins []string // allowed Origin values (default ports may be explicit); empty = same-host check only
	ConnsPerMinute int      // per client IP; 0 disables the limit
	TrustProxy     bool     // take client IP from X-Forwarded-For
	Release        string   // client release string shown to other users
	AllowSuperUser bool     // allow logging in as Mumble's SuperUser from the web
	AccessPassword string   // if set, browsers must send it in hello (separate from the Mumble server password)
	AdminPassword  string   // enables the /admin panel; admin sessions must send it in hello (empty = panel off)
	// ReservedSlots of MaxSessions can only be taken by browsers that send ReservedPassword (and by admin
	// sessions); everyone else shares MaxSessions-ReservedSlots. Both must be set, else the feature is off.
	ReservedSlots    int
	ReservedPassword string
	Logger           *slog.Logger
}

// Handler accepts browser WebSocket sessions on /ws.
type Handler struct {
	names      nameRegistry
	base       context.Context
	stop       context.CancelFunc
	cfg        Config
	log        *slog.Logger
	limiter    *ipLimiter
	fails      *failTracker
	adminFails *failTracker // separate from fails: admin guesses must not lock the main UI out
	slotFails  *failTracker // wrong reserved-slot passwords
	slotMu     sync.Mutex
	slotsGen   int // sessions in general slots
	slotsRes   int // sessions in reserved slots
	ipMu       sync.Mutex
	ipSessions map[string]int
	sessions   atomic.Int64
}

// New returns a bridge handler.
func New(cfg Config) *Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Release == "" {
		cfg.Release = "mumweb"
	}
	origins := make([]string, 0, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		origins = append(origins, normalizeOrigin(o))
	}
	cfg.AllowedOrigins = origins
	if cfg.ReservedSlots >= cfg.MaxSessions || cfg.ReservedPassword == "" {
		cfg.ReservedSlots = 0 // needs room for at least one general slot, and a password
	}
	base, stop := context.WithCancel(context.Background())
	return &Handler{base: base, stop: stop, cfg: cfg, log: cfg.Logger, limiter: newIPLimiter(cfg.ConnsPerMinute), fails: newFailTracker(), adminFails: newFailTracker(), slotFails: newFailTracker()}
}

// Shutdown ends all sessions. Hijacked WebSocket connections are not covered
// by http.Server.Shutdown, so call this from its shutdown path.
func (h *Handler) Shutdown() { h.stop() }

// AccessPasswordRequired reports whether browsers must supply the web access password.
func (h *Handler) AccessPasswordRequired() bool { return h.cfg.AccessPassword != "" }

// AdminEnabled reports whether the admin panel is switched on (MUMWEB_ADMIN_PASSWORD set).
func (h *Handler) AdminEnabled() bool { return h.cfg.AdminPassword != "" }

// ReservedSlotsEnabled reports whether some slots are kept for holders of the reserved-slot password.
func (h *Handler) ReservedSlotsEnabled() bool { return h.cfg.ReservedSlots > 0 }

// acquireSlot takes a web slot after login checks passed. Anyone may use the general slots
// (MaxSessions-ReservedSlots); privileged sessions (reserved password, admin) may also use the
// reserved ones. The returned func gives the slot back. Without reserved slots it always succeeds
// (the hard MaxSessions limit is applied when the socket is accepted).
func (h *Handler) acquireSlot(privileged bool) (func(), bool) {
	if h.cfg.ReservedSlots <= 0 {
		return func() {}, true
	}
	h.slotMu.Lock()
	defer h.slotMu.Unlock()
	if h.slotsGen < h.cfg.MaxSessions-h.cfg.ReservedSlots {
		h.slotsGen++
		return h.slotRelease(&h.slotsGen), true
	}
	if privileged && h.slotsRes < h.cfg.ReservedSlots {
		h.slotsRes++
		return h.slotRelease(&h.slotsRes), true
	}
	return nil, false
}

func (h *Handler) slotRelease(n *int) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			h.slotMu.Lock()
			*n--
			h.slotMu.Unlock()
		})
	}
}

// Sessions returns the number of open sessions.
func (h *Handler) Sessions() int { return int(h.sessions.Load()) }

// acquireIP counts a session for ip and reports whether it is within MaxPerIP.
func (h *Handler) acquireIP(ip string) bool {
	if h.cfg.MaxPerIP <= 0 {
		return true
	}
	h.ipMu.Lock()
	defer h.ipMu.Unlock()
	if h.ipSessions == nil {
		h.ipSessions = map[string]int{}
	}
	if h.ipSessions[ip] >= h.cfg.MaxPerIP {
		return false
	}
	h.ipSessions[ip]++
	return true
}

func (h *Handler) releaseIP(ip string) {
	if h.cfg.MaxPerIP <= 0 {
		return
	}
	h.ipMu.Lock()
	defer h.ipMu.Unlock()
	if h.ipSessions[ip]--; h.ipSessions[ip] <= 0 {
		delete(h.ipSessions, ip)
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, h.cfg.TrustProxy)
	log := h.log.With("ip", ip)

	opts := &websocket.AcceptOptions{}
	if len(h.cfg.AllowedOrigins) > 0 {
		origin := r.Header.Get("Origin")
		if !slices.Contains(h.cfg.AllowedOrigins, normalizeOrigin(origin)) {
			log.Warn("ws origin rejected", "origin", origin)
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		opts.InsecureSkipVerify = true // origin already checked above
	}
	if !h.limiter.allow(ip) {
		log.Warn("ws rate limited")
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	if n := h.sessions.Add(1); h.cfg.MaxSessions > 0 && n > int64(h.cfg.MaxSessions) {
		h.sessions.Add(-1)
		log.Warn("ws session limit reached", "max", h.cfg.MaxSessions)
		http.Error(w, "server full", http.StatusServiceUnavailable)
		return
	}
	defer h.sessions.Add(-1)
	if !h.acquireIP(ip) {
		log.Warn("ws per-ip session limit reached", "max", h.cfg.MaxPerIP)
		http.Error(w, "too many sessions from this address", http.StatusTooManyRequests)
		return
	}
	defer h.releaseIP(ip)

	ws, err := websocket.Accept(w, r, opts)
	if err != nil {
		log.Warn("ws accept failed", "err", err)
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(maxJSONSize)

	s := &session{h: h, ws: ws, log: log, ip: ip}
	code, reason := s.run(r.Context())
	_ = ws.Close(code, reason)
}

type session struct {
	ip  string
	h   *Handler
	ws  *websocket.Conn
	log *slog.Logger
	mc  *mumble.Client

	admin       bool        // admin panel session: no audio, admin commands allowed
	adminBucket tokenBucket // paces admin writes below the Mumble server's message rate limit
	bansMu      sync.Mutex
	lastBans    map[string]bool // keys of the ban list the server last sent (pump goroutine writes)
}

// run drives one session and returns the WebSocket close status.
func (s *session) run(ctx context.Context) (websocket.StatusCode, string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer context.AfterFunc(s.h.base, cancel)()

	hello, err := s.readHello(ctx)
	if err != nil {
		s.log.Info("ws session ended before hello", "err", err)
		return websocket.StatusPolicyViolation, "expected hello"
	}
	if hello.Role == roleAdmin {
		// The admin password replaces the web access password for this session.
		if reason, msg := s.checkAdmin(ctx, hello.AdminPassword); reason != "" {
			_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: reason, Message: msg})
			return websocket.StatusNormalClosure, "admin access denied"
		}
		s.admin = true
		s.ws.SetReadLimit(maxAdminJSONSize)
		s.log = s.log.With("role", roleAdmin)
	} else if reason, msg := s.checkAccess(ctx, hello.AccessPassword); reason != "" {
		_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: reason, Message: msg})
		return websocket.StatusNormalClosure, "access denied"
	}
	privileged := s.admin
	if !s.admin && s.h.cfg.ReservedSlots > 0 && hello.ReservedPassword != "" {
		if reason, msg := s.checkReserved(ctx, hello.ReservedPassword); reason != "" {
			_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: reason, Message: msg})
			return websocket.StatusNormalClosure, "reserved password denied"
		}
		privileged = true
	}
	releaseSlot, ok := s.h.acquireSlot(privileged)
	if !ok {
		s.log.Info("web slots full", "privileged", privileged)
		_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: rejectWebFull, Message: "all web slots are taken"})
		return websocket.StatusNormalClosure, "web slots full"
	}
	defer releaseSlot()
	username := strings.TrimSpace(hello.Username)
	if username == "" || len(username) > maxUsernameLen || !utf8.ValidString(username) {
		_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: "InvalidUsername", Message: "invalid username"})
		return websocket.StatusNormalClosure, "invalid username"
	}
	if !s.admin && !s.h.cfg.AllowSuperUser && strings.EqualFold(username, "SuperUser") {
		s.log.Warn("web SuperUser login refused (MUMWEB_ALLOW_SUPERUSER=false)")
		_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: "SuperUserDisabled", Message: "SuperUser login is disabled for the web client"})
		return websocket.StatusNormalClosure, "superuser disabled"
	}
	s.log = s.log.With("user", username)

	cert, err := s.identity(ctx, username, hello.Cert)
	if err != nil {
		s.log.Warn("certificate error", "err", err)
		_ = s.sendJSON(ctx, errorMsg{T: "error", Code: errCodeInvalidCert, Message: "invalid certificate"})
		return websocket.StatusPolicyViolation, "invalid certificate"
	}

	// All web users reach Mumble from the gateway's IP. Mumble lets an unregistered
	// newcomer take over a name in use when the IP matches ("reuse of name from same
	// IP") and kicks the old session, so any web user could kick another one by
	// picking their name. Only the same certificate (a reconnect) may take a name
	// over; anyone else gets UsernameInUse, as from a different IP.
	release, ok := s.h.names.claim(username, certHash(cert))
	if !ok {
		s.log.Info("username in use by another web session")
		_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: "UsernameInUse", Message: "username already in use"})
		return websocket.StatusNormalClosure, "username in use"
	}
	defer release()

	mc, err := mumble.Dial(ctx, mumble.Config{
		Addr:        s.h.cfg.MumbleAddr,
		Username:    username,
		Password:    hello.Password,
		Tokens:      cleanTokens(hello.Tokens),
		Certificate: cert,
		TLSVerify:   s.h.cfg.TLSVerify,
		Fingerprint: s.h.cfg.Fingerprint,
		Release:     s.h.cfg.Release,
		OS:          "Web",
		Logger:      s.log,
	})
	if err != nil {
		var rej *mumble.RejectError
		if errors.As(err, &rej) {
			s.log.Info("mumble login rejected", "type", rej.Type.String())
			_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: rej.Type.String(), Message: rej.Reason})
			return websocket.StatusNormalClosure, "rejected"
		}
		s.log.Warn("mumble connect failed", "err", err)
		_ = s.sendJSON(ctx, errorMsg{T: "error", Code: errCodeMumbleUnavailable, Message: "cannot reach Mumble server"})
		return websocket.StatusTryAgainLater, "mumble unavailable"
	}
	s.mc = mc
	defer mc.Close()
	s.log = s.log.With("session", mc.Session())
	s.log.Info("session ready")
	if s.admin {
		// The panel has no audio path; tell the server not to send any.
		_ = mc.SetSelf(true, true)
		s.adminBucket.take(time.Now(), adminPerS, adminBurst)
	}

	if err := s.sendInitialState(ctx); err != nil {
		return websocket.StatusGoingAway, ""
	}

	// Mumble -> browser in the background; browser -> Mumble here.
	pumpDone := make(chan error, 1)
	go func() { pumpDone <- s.pumpEvents(ctx) }()
	readErr := s.readLoop(ctx)
	cancel()
	mc.Close()
	pumpErr := <-pumpDone

	switch {
	case errors.Is(readErr, errProtocol):
		s.log.Warn("closing session", "err", readErr)
		return websocket.StatusPolicyViolation, "protocol error"
	case pumpErr != nil:
		s.log.Info("mumble connection ended", "err", pumpErr)
		return websocket.StatusTryAgainLater, "mumble disconnected"
	default:
		s.log.Info("session closed", "reason", readErr)
		return websocket.StatusNormalClosure, ""
	}
}

// checkAccess enforces MUMWEB_ACCESS_PASSWORD. It returns a reject reason (empty
// when access is granted). Wrong guesses are delayed and, after a few, the IP is
// locked out for a while even if it then sends the right password.
func (s *session) checkAccess(ctx context.Context, given string) (reason, msg string) {
	want := s.h.cfg.AccessPassword
	if want == "" {
		return "", ""
	}
	now := time.Now()
	if s.h.fails.locked(s.ip, now) {
		s.log.Warn("access password: ip locked out")
		return rejectAccessRateLimited, "too many wrong access passwords, try again later"
	}
	if given == "" {
		return rejectAccessRequired, "access password required"
	}
	if passwordMatches(given, want) {
		s.h.fails.ok(s.ip)
		return "", ""
	}
	s.h.fails.fail(s.ip, now)
	s.log.Warn("access password: wrong password")
	select {
	case <-time.After(accessFailDelay):
	case <-ctx.Done():
	}
	return rejectAccessWrong, "wrong access password"
}

// checkReserved verifies the reserved-slot password with the same delay and lockout as the access password.
func (s *session) checkReserved(ctx context.Context, given string) (reason, msg string) {
	now := time.Now()
	if s.h.slotFails.locked(s.ip, now) {
		s.log.Warn("reserved password: ip locked out")
		return rejectReservedRateLimited, "too many wrong reserved-slot passwords, try again later"
	}
	if passwordMatches(given, s.h.cfg.ReservedPassword) {
		s.h.slotFails.ok(s.ip)
		return "", ""
	}
	s.h.slotFails.fail(s.ip, now)
	s.log.Warn("reserved password: wrong password")
	select {
	case <-time.After(accessFailDelay):
	case <-ctx.Done():
	}
	return rejectReservedWrong, "wrong reserved-slot password"
}

func (s *session) readHello(ctx context.Context) (clientMsg, error) {
	ctx, cancel := context.WithTimeout(ctx, helloTimeout)
	defer cancel()
	typ, data, err := s.ws.Read(ctx)
	if err != nil {
		return clientMsg{}, err
	}
	var m clientMsg
	if typ != websocket.MessageText || json.Unmarshal(data, &m) != nil || m.T != "hello" {
		return clientMsg{}, errors.New("first message is not hello")
	}
	return m, nil
}

// identity loads the browser's certificate, or generates one and sends it back.
func (s *session) identity(ctx context.Context, username string, pair *certPair) (*tls.Certificate, error) {
	if pair != nil {
		return certs.Load([]byte(pair.PEM), []byte(pair.Key))
	}
	certPEM, keyPEM, err := certs.Generate(username)
	if err != nil {
		return nil, err
	}
	s.log.Info("generated client certificate")
	if err := s.sendJSON(ctx, certMsg{T: "cert", PEM: string(certPEM), Key: string(keyPEM)}); err != nil {
		return nil, err
	}
	return certs.Load(certPEM, keyPEM)
}

// sendInitialState sends the channel tree and users, then "ready".
func (s *session) sendInitialState(ctx context.Context) error {
	st := s.mc.State()
	for _, c := range st.Channels() {
		if err := s.sendJSON(ctx, newChannelMsg(c)); err != nil {
			return err
		}
	}
	for _, u := range st.Users() {
		if err := s.sendJSON(ctx, newUserMsg(u)); err != nil {
			return err
		}
	}
	info := s.mc.Info()
	return s.sendJSON(ctx, readyMsg{
		T: "ready", Session: info.Session, Permissions: info.Permissions, Welcome: info.WelcomeText,
		MaxBandwidth: info.MaxBandwidth, AllowHTML: info.AllowHTML, MessageLen: info.MessageLength,
	})
}

// pumpEvents forwards Mumble events to the browser until the Mumble
// connection ends (returns its cause) or ctx is cancelled (returns nil).
func (s *session) pumpEvents(ctx context.Context) error {
	for ev := range s.mc.Events() {
		var err error
		switch e := ev.(type) {
		case mumble.AudioEvent:
			if !s.admin {
				err = s.write(ctx, websocket.MessageBinary, encodeAudioFrame(e.Packet))
			}
		case mumble.ChannelEvent:
			err = s.sendJSON(ctx, newChannelMsg(e.Channel))
		case mumble.ChannelRemoveEvent:
			err = s.sendJSON(ctx, channelRemoveMsg{T: "channelRemove", ID: e.ID})
		case mumble.UserEvent:
			err = s.sendJSON(ctx, newUserMsg(e.User))
		case mumble.UserRemoveEvent:
			err = s.sendJSON(ctx, userRemoveMsg{T: "userRemove", Session: e.Session, Actor: e.Actor, Reason: e.Reason, Ban: e.Ban})
		case mumble.TextEvent:
			err = s.sendJSON(ctx, newTextMsg(e))
		case mumble.DeniedEvent:
			err = s.sendJSON(ctx, deniedMsg{T: "denied", Type: e.Type.String(), Reason: e.Reason,
				Permission: e.Permission, ChannelID: e.ChannelID, Session: e.Session})
		case mumble.ChannelPasswordEvent:
			err = s.sendJSON(ctx, channelPasswordMsg{T: "channelPassword", ChannelID: e.ChannelID, Password: e.Password})
		case mumble.PermissionEvent:
			err = s.sendJSON(ctx, permissionsMsg{T: "permissions", ChannelID: e.ChannelID, Permissions: e.Permissions, Flush: e.Flush})
		case mumble.UserStatsEvent:
			err = s.sendJSON(ctx, newUserStatsMsg(e.Stats))
		case mumble.BanListEvent:
			s.rememberBans(e.Bans)
			err = s.sendJSON(ctx, newBanListMsg(e.Bans))
		case mumble.UserListEvent:
			err = s.sendJSON(ctx, newUserListMsg(e.Users))
		case mumble.ACLEvent:
			err = s.sendJSON(ctx, newACLMsg(e.ACL))
		case mumble.ServerConfigEvent:
			// Already reflected in "ready"; later changes are rare and not forwarded.
		}
		if err != nil {
			return nil // browser side failed; readLoop will notice and clean up
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	err := s.mc.Err()
	var rej *mumble.RejectError
	if errors.As(err, &rej) {
		_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: rej.Type.String(), Message: rej.Reason})
	} else {
		_ = s.sendJSON(ctx, errorMsg{T: "error", Code: errCodeDisconnected, Message: "disconnected from Mumble server"})
	}
	// Unblock readLoop so the session ends.
	_ = s.ws.Close(websocket.StatusTryAgainLater, "mumble disconnected")
	return fmt.Errorf("mumble: %w", err)
}

var errProtocol = errors.New("bridge: protocol violation")

// readLoop handles browser messages until the WebSocket closes or errs.
func (s *session) readLoop(ctx context.Context) error {
	var audio, control tokenBucket
	dropped, controlDropped := 0, 0
	for {
		rctx, cancel := context.WithTimeout(ctx, browserIdle)
		typ, data, err := s.ws.Read(rctx)
		cancel()
		if err != nil {
			return err
		}
		if typ == websocket.MessageBinary {
			if s.admin {
				return fmt.Errorf("%w: audio frame on an admin session", errProtocol)
			}
			p, err := parseAudioFrame(data)
			if err != nil {
				return fmt.Errorf("%w: %v (len %d)", errProtocol, err, len(data))
			}
			if !audio.take(time.Now(), audioPacketsPerS, audioPacketsBurst) {
				if dropped++; dropped%100 == 1 {
					s.log.Warn("audio rate limit exceeded; dropping packets", "dropped", dropped)
				}
				continue
			}
			if err := s.mc.SendAudio(p); err != nil {
				return err
			}
			continue
		}

		if !control.take(time.Now(), controlPerS, controlBurst) {
			if controlDropped++; controlDropped == 1 {
				s.log.Warn("control message rate limit exceeded; dropping messages")
			}
			if controlDropped >= controlFloodLimit {
				return fmt.Errorf("%w: control message flood", errProtocol)
			}
			continue
		}
		controlDropped = 0
		var m clientMsg
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("%w: bad JSON: %v", errProtocol, err)
		}
		if err := s.handleControl(ctx, m); err != nil {
			return err
		}
	}
}

func (s *session) handleControl(ctx context.Context, m clientMsg) error {
	if s.admin && m.T != "ping" {
		s.audit(m)
		if !s.adminPace(ctx) {
			s.log.Warn("admin command dropped by pacing", "action", m.T)
			return s.sendJSON(ctx, adminBusyMsg{T: "adminBusy"})
		}
	}
	switch m.T {
	case "adminBanList", "adminSetBanList", "adminUserList", "adminUpdateUsers", "adminACL", "adminSetACL":
		if !s.admin {
			return fmt.Errorf("%w: %s on a non-admin session", errProtocol, m.T)
		}
		return s.adminAction(ctx, m)
	case "joinChannel":
		if m.ChannelID == nil {
			return fmt.Errorf("%w: joinChannel without channelId", errProtocol)
		}
		return s.mc.JoinChannel(*m.ChannelID)
	case "createChannel":
		name := strings.TrimSpace(m.Name)
		if m.Parent == nil || name == "" || len(name) > maxChannelNameLen || !utf8.ValidString(name) {
			return fmt.Errorf("%w: bad createChannel", errProtocol)
		}
		return s.mc.CreateChannel(*m.Parent, name, m.Temporary)
	case "setSelf":
		u, _ := s.mc.State().User(s.mc.Session())
		mute, deaf := u.SelfMute, u.SelfDeaf
		if m.Mute != nil {
			mute = *m.Mute
		}
		if m.Deaf != nil {
			deaf = *m.Deaf
		}
		return s.mc.SetSelf(mute, deaf)
	case "text":
		if m.Message == "" {
			return nil
		}
		if m.Session != nil { // private message
			return s.mc.SendText(m.Message, nil, []uint32{*m.Session})
		}
		if m.ChannelID != nil { // to a specific channel, optionally with sub-channels
			if m.Tree {
				return s.mc.SendTreeText(m.Message, *m.ChannelID)
			}
			return s.mc.SendText(m.Message, []uint32{*m.ChannelID}, nil)
		}
		u, _ := s.mc.State().User(s.mc.Session())
		return s.mc.SendText(m.Message, []uint32{u.ChannelID}, nil)
	case "queryPermissions":
		if m.ChannelID == nil {
			return fmt.Errorf("%w: queryPermissions without channelId", errProtocol)
		}
		return s.mc.QueryPermissions(*m.ChannelID)
	case "setTokens":
		return s.mc.SetAccessTokens(cleanTokens(m.Tokens))
	case "editChannel", "removeChannel", "linkChannels", "listen", "requestDescription", "setChannelPassword", "queryChannelPassword":
		return s.channelAction(m)
	case "moveUser", "setUser", "kick", "ban", "register", "resetComment", "resetTexture", "requestComment", "userStats":
		return s.userAction(m)
	case "ping":
		var ts float64
		if m.TS != nil {
			ts = *m.TS
		}
		return s.sendJSON(ctx, pongMsg{T: "pong", TS: ts})
	case "hello":
		return fmt.Errorf("%w: duplicate hello", errProtocol)
	default:
		s.log.Debug("ignoring unknown message", "t", m.T)
		return nil
	}
}

// channelAction handles channel management commands; the server checks permissions.
func (s *session) channelAction(m clientMsg) error {
	if m.ChannelID == nil {
		return fmt.Errorf("%w: %s without channelId", errProtocol, m.T)
	}
	id := *m.ChannelID
	switch m.T {
	case "editChannel":
		e := mumble.ChannelEdit{Description: m.Description, Position: m.Position, MaxUsers: m.MaxUsers}
		if m.NewName != nil {
			name := strings.TrimSpace(*m.NewName)
			if name == "" || len(name) > maxChannelNameLen || !utf8.ValidString(name) {
				return fmt.Errorf("%w: bad channel name", errProtocol)
			}
			e.Name = &name
		}
		if e.Description != nil && len(*e.Description) > maxJSONSize {
			return fmt.Errorf("%w: description too long", errProtocol)
		}
		return s.mc.EditChannel(id, e)
	case "removeChannel":
		s.log.Info("channel removal requested", "channel", id)
		return s.mc.RemoveChannel(id)
	case "linkChannels":
		return s.mc.LinkChannels(id, m.Add, m.Remove)
	case "listen":
		if m.On != nil && *m.On {
			return s.mc.SetListening([]uint32{id}, nil)
		}
		return s.mc.SetListening(nil, []uint32{id})
	case "requestDescription":
		return s.mc.RequestChannelDescription(id)
	case "queryChannelPassword":
		return s.mc.QueryChannelPassword(id)
	case "setChannelPassword":
		if m.ChannelPass == nil || len(*m.ChannelPass) > maxTokenLen || !utf8.ValidString(*m.ChannelPass) {
			return fmt.Errorf("%w: bad channel password", errProtocol)
		}
		s.log.Info("channel password change requested", "channel", id)
		return s.mc.SetChannelPassword(id, strings.TrimSpace(*m.ChannelPass))
	}
	return nil
}

// cleanTokens bounds the access tokens a browser may send.
func cleanTokens(in []string) []string {
	var out []string
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" || len(t) > maxTokenLen || !utf8.ValidString(t) || slices.Contains(out, t) {
			continue
		}
		out = append(out, t)
		if len(out) == maxTokens {
			break
		}
	}
	return out
}

// userAction handles commands that target another user. The Mumble server
// enforces permissions; denials come back as "denied".
func (s *session) userAction(m clientMsg) error {
	if m.Session == nil {
		return fmt.Errorf("%w: %s without session", errProtocol, m.T)
	}
	target := *m.Session
	reason := strings.TrimSpace(m.Reason)
	if len(reason) > maxReasonLen {
		reason = reason[:maxReasonLen]
	}
	switch m.T {
	case "moveUser":
		if m.ChannelID == nil {
			return fmt.Errorf("%w: moveUser without channelId", errProtocol)
		}
		return s.mc.UpdateUser(target, mumble.UserUpdate{ChannelID: m.ChannelID})
	case "setUser":
		return s.mc.UpdateUser(target, mumble.UserUpdate{Mute: m.Mute, Deaf: m.Deaf, PrioritySpeaker: m.PrioritySpeaker})
	case "kick":
		s.log.Info("kick requested", "target", target)
		return s.mc.Kick(target, reason)
	case "ban":
		s.log.Info("ban requested (certificate only)", "target", target)
		if err := s.mc.BanByCertificate(target, reason); err != nil {
			s.log.Warn("ban failed", "err", err)
		}
		return nil
	case "register":
		return s.mc.UpdateUser(target, mumble.UserUpdate{Register: true})
	case "resetComment":
		return s.mc.UpdateUser(target, mumble.UserUpdate{ResetComment: true})
	case "resetTexture":
		return s.mc.UpdateUser(target, mumble.UserUpdate{ResetTexture: true})
	case "requestComment":
		return s.mc.RequestComment(target)
	case "userStats":
		return s.mc.RequestUserStats(target)
	}
	return nil
}

func (s *session) sendJSON(ctx context.Context, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.write(ctx, websocket.MessageText, data)
}

func (s *session) write(ctx context.Context, typ websocket.MessageType, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	defer cancel()
	return s.ws.Write(ctx, typ, data)
}

// certHash is Mumble's certificate hash: hex SHA-1 of the DER leaf certificate.
func certHash(c *tls.Certificate) string {
	if c == nil || len(c.Certificate) == 0 {
		return ""
	}
	sum := sha1.Sum(c.Certificate[0])
	return hex.EncodeToString(sum[:])
}

// nameRegistry tracks which certificate uses each (lower-cased) name among the
// gateway's live sessions.
type nameRegistry struct {
	mu    sync.Mutex
	names map[string]*nameClaim
}

type nameClaim struct {
	hash string
	refs int
}

// claim reserves name for hash. It fails if another certificate holds the name.
// The returned function releases the claim.
func (r *nameRegistry) claim(name, hash string) (release func(), ok bool) {
	key := strings.ToLower(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.names == nil {
		r.names = map[string]*nameClaim{}
	}
	c := r.names[key]
	if c != nil && c.hash != hash {
		return nil, false
	}
	if c == nil {
		c = &nameClaim{hash: hash}
		r.names[key] = c
	}
	c.refs++
	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			if c.refs--; c.refs == 0 && r.names[key] == c {
				delete(r.names, key)
			}
		})
	}, true
}
