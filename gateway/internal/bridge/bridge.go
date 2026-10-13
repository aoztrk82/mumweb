package bridge

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
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
	audioPacketsPerS  = 100
	audioPacketsBurst = 150
)

// Config configures the WebSocket bridge.
type Config struct {
	MumbleAddr     string
	TLSVerify      bool
	Fingerprint    string
	MaxSessions    int
	AllowedOrigins []string // exact Origin values; empty = same-host check only
	ConnsPerMinute int      // per client IP; 0 disables the limit
	TrustProxy     bool     // take client IP from X-Forwarded-For
	Release        string   // client release string shown to other users
	Logger         *slog.Logger
}

// Handler accepts browser WebSocket sessions on /ws.
type Handler struct {
	base     context.Context
	stop     context.CancelFunc
	cfg      Config
	log      *slog.Logger
	limiter  *ipLimiter
	sessions atomic.Int64
}

// New returns a bridge handler.
func New(cfg Config) *Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Release == "" {
		cfg.Release = "mumweb"
	}
	base, stop := context.WithCancel(context.Background())
	return &Handler{base: base, stop: stop, cfg: cfg, log: cfg.Logger, limiter: newIPLimiter(cfg.ConnsPerMinute)}
}

// Shutdown ends all sessions. Hijacked WebSocket connections are not covered
// by http.Server.Shutdown, so call this from its shutdown path.
func (h *Handler) Shutdown() { h.stop() }

// Sessions returns the number of open sessions.
func (h *Handler) Sessions() int { return int(h.sessions.Load()) }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, h.cfg.TrustProxy)
	log := h.log.With("ip", ip)

	opts := &websocket.AcceptOptions{}
	if len(h.cfg.AllowedOrigins) > 0 {
		origin := r.Header.Get("Origin")
		if !slices.Contains(h.cfg.AllowedOrigins, origin) {
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

	ws, err := websocket.Accept(w, r, opts)
	if err != nil {
		log.Warn("ws accept failed", "err", err)
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(maxJSONSize)

	s := &session{h: h, ws: ws, log: log}
	code, reason := s.run(r.Context())
	_ = ws.Close(code, reason)
}

type session struct {
	h   *Handler
	ws  *websocket.Conn
	log *slog.Logger
	mc  *mumble.Client
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
	username := strings.TrimSpace(hello.Username)
	if username == "" || len(username) > maxUsernameLen || !utf8.ValidString(username) {
		_ = s.sendJSON(ctx, rejectMsg{T: "reject", Reason: "InvalidUsername", Message: "invalid username"})
		return websocket.StatusNormalClosure, "invalid username"
	}
	s.log = s.log.With("user", username)

	cert, err := s.identity(ctx, username, hello.Cert)
	if err != nil {
		s.log.Warn("certificate error", "err", err)
		_ = s.sendJSON(ctx, errorMsg{T: "error", Code: errCodeInvalidCert, Message: "invalid certificate"})
		return websocket.StatusPolicyViolation, "invalid certificate"
	}

	mc, err := mumble.Dial(ctx, mumble.Config{
		Addr:        s.h.cfg.MumbleAddr,
		Username:    username,
		Password:    hello.Password,
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
		T: "ready", Session: info.Session, Welcome: info.WelcomeText,
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
			err = s.write(ctx, websocket.MessageBinary, encodeAudioFrame(e.Packet))
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
			err = s.sendJSON(ctx, deniedMsg{T: "denied", Type: e.Type.String(), Reason: e.Reason})
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
	var audio tokenBucket
	dropped := 0
	for {
		rctx, cancel := context.WithTimeout(ctx, browserIdle)
		typ, data, err := s.ws.Read(rctx)
		cancel()
		if err != nil {
			return err
		}
		if typ == websocket.MessageBinary {
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
	switch m.T {
	case "joinChannel":
		if m.ChannelID == nil {
			return fmt.Errorf("%w: joinChannel without channelId", errProtocol)
		}
		return s.mc.JoinChannel(*m.ChannelID)
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
		u, _ := s.mc.State().User(s.mc.Session())
		return s.mc.SendText(m.Message, []uint32{u.ChannelID}, nil)
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
