// Command mumprobe is a CLI for probing a Mumble server directly (no browser).
//
// It connects, prints the channel tree and users, then logs events (state
// changes, text messages, voice packets) until interrupted.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aoztrk82/mumweb/gateway/internal/mumble"
)

// opusSilence is a valid 20 ms Opus packet (CELT-only, 48 kHz) that decodes to silence.
var opusSilence = []byte{0xf8, 0xff, 0xfe}

func main() {
	addr := flag.String("addr", "localhost:64738", "Mumble server host:port")
	user := flag.String("user", "mumprobe", "username")
	pass := flag.String("password", "", "server password")
	certFile := flag.String("cert", "", "client certificate PEM (optional)")
	keyFile := flag.String("key", "", "client private key PEM (defaults to -cert)")
	verify := flag.Bool("verify", false, "verify the server TLS certificate")
	fingerprint := flag.String("fingerprint", "", "pin server certificate SHA-256 fingerprint")
	say := flag.String("say", "", "send this text message to the current channel after connecting")
	join := flag.Int("join", -1, "move to this channel ID after connecting")
	tx := flag.Duration("tx", 0, "transmit Opus silence for this long (e.g. 2s) to exercise the voice path")
	exit := flag.Bool("exit", false, "exit after the initial dump and -say/-join/-tx actions")
	debug := flag.Bool("debug", false, "debug logging")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	cfg := mumble.Config{
		Addr:        *addr,
		Username:    *user,
		Password:    *pass,
		TLSVerify:   *verify,
		Fingerprint: *fingerprint,
		Release:     "mumprobe",
		OS:          "CLI",
		Logger:      log,
	}
	if *certFile != "" {
		kf := *keyFile
		if kf == "" {
			kf = *certFile
		}
		cert, err := tls.LoadX509KeyPair(*certFile, kf)
		if err != nil {
			fatal("load certificate", err)
		}
		cfg.Certificate = &cert
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c, err := mumble.Dial(ctx, cfg)
	if err != nil {
		fatal("connect", err)
	}
	defer c.Close()

	info := c.Info()
	fmt.Printf("connected: session=%d maxBandwidth=%d server=%s (v2=%#x)\n",
		info.Session, info.MaxBandwidth, info.ServerRelease, info.ServerVersion)
	if info.WelcomeText != "" {
		fmt.Printf("welcome: %s\n", info.WelcomeText)
	}
	printTree(c.State())

	if *join >= 0 {
		if err := c.JoinChannel(uint32(*join)); err != nil {
			fatal("join", err)
		}
	}
	if *say != "" {
		if err := c.SendText(*say, []uint32{currentChannel(c)}, nil); err != nil {
			fatal("say", err)
		}
		fmt.Printf("sent text to channel %d\n", currentChannel(c))
	}
	if *tx > 0 {
		go transmit(c, *tx)
	}

	var deadline <-chan time.Time
	if *exit {
		// Give the server time to process our actions (and -tx to finish).
		deadline = time.After(*tx + time.Second)
	}

	speaking := map[uint32]int{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline:
			return
		case ev, ok := <-c.Events():
			if !ok {
				if err := c.Err(); err != nil && !errors.Is(err, mumble.ErrClosed) {
					fatal("disconnected", err)
				}
				return
			}
			logEvent(c, ev, speaking)
		}
	}
}

func transmit(c *mumble.Client, d time.Duration) {
	frames := int(d / (20 * time.Millisecond))
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for i := range frames {
		<-tick.C
		err := c.SendAudio(mumble.AudioPacket{
			FrameNumber: uint64(i),
			Opus:        opusSilence,
			Terminator:  i == frames-1,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "tx: %v\n", err)
			return
		}
	}
	fmt.Printf("tx: sent %d frames\n", frames)
}

func logEvent(c *mumble.Client, ev mumble.Event, speaking map[uint32]int) {
	ts := time.Now().Format("15:04:05.000")
	switch e := ev.(type) {
	case mumble.AudioEvent:
		p := e.Packet
		// Log the first packet of a burst, every 50th, and the terminator.
		n := speaking[p.Sender]
		if n == 0 || n%50 == 0 || p.Terminator {
			fmt.Printf("%s audio from=%d(%s) frame=%d bytes=%d ctx=%d vol=%g term=%v\n",
				ts, p.Sender, userName(c, p.Sender), p.FrameNumber, len(p.Opus), p.Context, p.VolumeAdjustment, p.Terminator)
		}
		if p.Terminator {
			delete(speaking, p.Sender)
		} else {
			speaking[p.Sender] = n + 1
		}
	case mumble.TextEvent:
		fmt.Printf("%s text from=%d(%s) channels=%v sessions=%v: %s\n",
			ts, e.Actor, userName(c, e.Actor), e.Channels, e.Sessions, e.Message)
	case mumble.UserEvent:
		u := e.User
		fmt.Printf("%s user %d %q channel=%d mute=%v deaf=%v selfMute=%v selfDeaf=%v registered=%v\n",
			ts, u.Session, u.Name, u.ChannelID, u.Mute, u.Deaf, u.SelfMute, u.SelfDeaf, u.Registered)
	case mumble.UserRemoveEvent:
		fmt.Printf("%s userRemove %d reason=%q ban=%v\n", ts, e.Session, e.Reason, e.Ban)
	case mumble.ChannelEvent:
		fmt.Printf("%s channel %d %q parent=%d\n", ts, e.Channel.ID, e.Channel.Name, e.Channel.Parent)
	case mumble.ChannelRemoveEvent:
		fmt.Printf("%s channelRemove %d\n", ts, e.ID)
	case mumble.DeniedEvent:
		fmt.Printf("%s denied type=%s reason=%q\n", ts, e.Type, e.Reason)
	case mumble.ServerConfigEvent:
		fmt.Printf("%s serverConfig maxBandwidth=%d allowHTML=%v msgLen=%d\n",
			ts, e.Info.MaxBandwidth, e.Info.AllowHTML, e.Info.MessageLength)
	}
}

func printTree(s *mumble.State) {
	channels := s.Channels()
	users := s.Users()
	children := map[uint32][]mumble.Channel{}
	for _, ch := range channels {
		if ch.ID != 0 {
			children[ch.Parent] = append(children[ch.Parent], ch)
		}
	}
	var walk func(ch mumble.Channel, depth int)
	walk = func(ch mumble.Channel, depth int) {
		indent := strings.Repeat("  ", depth)
		fmt.Printf("%s#%d %s\n", indent, ch.ID, ch.Name)
		for _, u := range users {
			if u.ChannelID == ch.ID {
				flags := ""
				if u.Registered {
					flags += " [registered]"
				}
				if u.SelfMute || u.Mute {
					flags += " [muted]"
				}
				if u.SelfDeaf || u.Deaf {
					flags += " [deaf]"
				}
				fmt.Printf("%s  - %s (session %d)%s\n", indent, u.Name, u.Session, flags)
			}
		}
		for _, sub := range children[ch.ID] {
			walk(sub, depth+1)
		}
	}
	for _, ch := range channels {
		if ch.ID == 0 {
			walk(ch, 0)
		}
	}
}

func currentChannel(c *mumble.Client) uint32 {
	u, _ := c.State().User(c.Session())
	return u.ChannelID
}

func userName(c *mumble.Client, session uint32) string {
	u, ok := c.State().User(session)
	if !ok {
		return "?"
	}
	return u.Name
}

func fatal(what string, err error) {
	fmt.Fprintf(os.Stderr, "mumprobe: %s: %v\n", what, err)
	os.Exit(1)
}
