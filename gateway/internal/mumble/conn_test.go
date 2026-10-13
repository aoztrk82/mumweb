package mumble

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/aoztrk82/mumweb/gateway/internal/certs"
	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

// fakeServer is a minimal TLS Mumble server for one connection.
type fakeServer struct {
	t    *testing.T
	ln   net.Listener
	conn chan *serverConn
}

type serverConn struct {
	t *testing.T
	c net.Conn
	r *bufio.Reader
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	certPEM, keyPEM, err := certs.Generate("fake-server")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := certs.Load(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{*cert}})
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeServer{t: t, ln: ln, conn: make(chan *serverConn, 1)}
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		// Drive the TLS handshake now; the client may abort it (e.g. fingerprint mismatch).
		_ = c.(*tls.Conn).Handshake()
		s.conn <- &serverConn{t: t, c: c, r: bufio.NewReader(c)}
	}()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *fakeServer) accept() *serverConn {
	select {
	case c := <-s.conn:
		s.t.Cleanup(func() { c.c.Close() })
		return c
	case <-time.After(5 * time.Second):
		s.t.Fatal("no connection")
		return nil
	}
}

func (c *serverConn) expect(want MsgType) []byte {
	c.t.Helper()
	_ = c.c.SetReadDeadline(time.Now().Add(5 * time.Second))
	typ, payload, err := readFrame(c.r)
	if err != nil {
		c.t.Fatalf("server read: %v", err)
	}
	if typ != want {
		c.t.Fatalf("server got %s, want %s", typ, want)
	}
	return payload
}

func (c *serverConn) send(t MsgType, m proto.Message) {
	c.t.Helper()
	var payload []byte
	if m != nil {
		var err error
		if payload, err = proto.Marshal(m); err != nil {
			c.t.Fatal(err)
		}
	}
	if err := writeFrame(c.c, t, payload); err != nil {
		c.t.Fatalf("server write: %v", err)
	}
}

// handshake reads Version + Authenticate and returns the parsed messages.
func (c *serverConn) handshake() (*pb.Version, *pb.Authenticate) {
	c.t.Helper()
	var v pb.Version
	var a pb.Authenticate
	if err := proto.Unmarshal(c.expect(MsgVersion), &v); err != nil {
		c.t.Fatal(err)
	}
	if err := proto.Unmarshal(c.expect(MsgAuthenticate), &a); err != nil {
		c.t.Fatal(err)
	}
	return &v, &a
}

func (c *serverConn) sync(session uint32) {
	c.send(MsgCryptSetup, &pb.CryptSetup{Key: []byte{1}})
	c.send(MsgCodecVersion, &pb.CodecVersion{Alpha: proto.Int32(0), Beta: proto.Int32(0), PreferAlpha: proto.Bool(true), Opus: proto.Bool(true)})
	c.send(MsgChannelState, &pb.ChannelState{ChannelId: proto.Uint32(0), Name: proto.String("Root")})
	c.send(MsgChannelState, &pb.ChannelState{ChannelId: proto.Uint32(1), Parent: proto.Uint32(0), Name: proto.String("Oyun")})
	c.send(MsgUserState, &pb.UserState{Session: proto.Uint32(session), Name: proto.String("me"), ChannelId: proto.Uint32(1)})
	c.send(MsgServerSync, &pb.ServerSync{Session: proto.Uint32(session), MaxBandwidth: proto.Uint32(72000), WelcomeText: proto.String("hi")})
	c.send(MsgServerConfig, &pb.ServerConfig{AllowHtml: proto.Bool(true), MessageLength: proto.Uint32(5000)})
}

func dialFake(t *testing.T, s *fakeServer, cfg Config) (*Client, error) {
	t.Helper()
	cfg.Addr = s.ln.Addr().String()
	if cfg.Username == "" {
		cfg.Username = "me"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return Dial(ctx, cfg)
}

func TestDialHandshake(t *testing.T) {
	s := newFakeServer(t)
	type result struct {
		c   *Client
		err error
	}
	done := make(chan result, 1)
	go func() {
		c, err := dialFake(t, s, Config{Username: "Ahmet", Password: "pw", Tokens: []string{"tok"}})
		done <- result{c, err}
	}()

	sc := s.accept()
	v, a := sc.handshake()
	if v.GetVersionV2() != 1<<48|5<<32 || v.GetVersionV1() != 1<<16|5<<8 || v.GetOs() != "Web" {
		t.Fatalf("version %v", v)
	}
	if a.GetUsername() != "Ahmet" || a.GetPassword() != "pw" || !a.GetOpus() || len(a.Tokens) != 1 {
		t.Fatalf("authenticate %v", a)
	}
	// Unknown message types before sync must be skipped, not fatal.
	if err := writeFrame(sc.c, MsgType(99), []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	sc.sync(7)

	r := <-done
	if r.err != nil {
		t.Fatal(r.err)
	}
	c := r.c
	defer c.Close()
	info := c.Info()
	if c.Session() != 7 || info.MaxBandwidth != 72000 || info.WelcomeText != "hi" || !info.AllowHTML || info.MessageLength != 5000 {
		t.Fatalf("info %+v", info)
	}
	if ch := c.State().Channels(); len(ch) != 2 || ch[1].Name != "Oyun" {
		t.Fatalf("channels %+v", ch)
	}
	if u, ok := c.State().User(7); !ok || u.ChannelID != 1 {
		t.Fatalf("self %+v", u)
	}

	// After sync: events are published, unknown types still ignored.
	_ = writeFrame(sc.c, MsgType(42), nil)
	sc.send(MsgUserState, &pb.UserState{Session: proto.Uint32(8), Name: proto.String("Mehmet")})
	sc.send(MsgTextMessage, &pb.TextMessage{Actor: proto.Uint32(8), ChannelId: []uint32{1}, Message: proto.String("selam")})
	if ev := nextEvent(t, c); ev.(UserEvent).User.Name != "Mehmet" {
		t.Fatalf("got %#v", ev)
	}
	if ev := nextEvent(t, c); ev.(TextEvent).Message != "selam" {
		t.Fatalf("got %#v", ev)
	}

	// Outgoing commands are framed correctly.
	if err := c.JoinChannel(0); err != nil {
		t.Fatal(err)
	}
	var us pb.UserState
	_ = proto.Unmarshal(sc.expect(MsgUserState), &us)
	if us.GetSession() != 7 || us.ChannelId == nil || us.GetChannelId() != 0 {
		t.Fatalf("join %v", &us)
	}
	if err := c.SetSelf(false, true); err != nil {
		t.Fatal(err)
	}
	_ = proto.Unmarshal(sc.expect(MsgUserState), &us)
	if !us.GetSelfDeaf() || !us.GetSelfMute() {
		t.Fatalf("deaf must imply mute: %v", &us)
	}
	if err := c.SendAudio(AudioPacket{FrameNumber: 4, Opus: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if p := sc.expect(MsgUDPTunnel); p[0] != udpTypeAudio {
		t.Fatalf("tunnel header %#x", p[0])
	}

	// Server closing the connection ends the event stream with an error.
	sc.c.Close()
	for range c.Events() {
	}
	if c.Err() == nil || errors.Is(c.Err(), ErrClosed) {
		t.Fatalf("err = %v", c.Err())
	}
}

func TestDialReject(t *testing.T) {
	s := newFakeServer(t)
	go func() {
		sc := s.accept()
		sc.handshake()
		sc.send(MsgReject, &pb.Reject{Type: pb.Reject_WrongServerPW.Enum(), Reason: proto.String("nope")})
	}()
	_, err := dialFake(t, s, Config{})
	var rej *RejectError
	if !errors.As(err, &rej) || rej.Type != pb.Reject_WrongServerPW || rej.Reason != "nope" {
		t.Fatalf("err = %v", err)
	}
}

func TestDialHandshakeTimeout(t *testing.T) {
	s := newFakeServer(t)
	go func() {
		sc := s.accept()
		sc.handshake() // ...and never answer
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Dial(ctx, Config{Addr: s.ln.Addr().String(), Username: "me"})
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("err = %v after %v", err, time.Since(start))
	}
}

func TestFingerprintMismatch(t *testing.T) {
	s := newFakeServer(t)
	_, err := dialFake(t, s, Config{Fingerprint: "00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff"})
	if err == nil || !strings.Contains(err.Error(), "fingerprint mismatch") {
		t.Fatalf("err = %v", err)
	}
	if _, err := dialFake(t, s, Config{Fingerprint: "zz"}); err == nil {
		t.Fatal("invalid fingerprint accepted")
	}
}

func TestCloseReleasesGoroutines(t *testing.T) {
	base := runtime.NumGoroutine()
	s := newFakeServer(t)
	go func() {
		sc := s.accept()
		sc.handshake()
		sc.sync(1)
	}()
	c, err := dialFake(t, s, Config{})
	if err != nil {
		t.Fatal(err)
	}
	// A stalled consumer must not block Close.
	_ = c.Close()
	if !errors.Is(c.Err(), ErrClosed) {
		t.Fatalf("err = %v", c.Err())
	}
	s.ln.Close()
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > base+1 { // +1: fake server's conn may linger until cleanup
		if time.Now().After(deadline) {
			t.Fatalf("goroutines: %d > %d", runtime.NumGoroutine(), base+1)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func nextEvent(t *testing.T, c *Client) Event {
	t.Helper()
	select {
	case ev, ok := <-c.Events():
		if !ok {
			t.Fatalf("events closed: %v", c.Err())
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
		return nil
	}
}
