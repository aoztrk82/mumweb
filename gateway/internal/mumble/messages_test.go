package mumble

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := writeFrame(&buf, MsgTextMessage, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := writeFrame(&buf, MsgPing, nil); err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 11, 0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o', 0, 3, 0, 0, 0, 0}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("encoded %x, want %x", buf.Bytes(), want)
	}

	typ, payload, err := readFrame(&buf)
	if err != nil || typ != MsgTextMessage || string(payload) != "hello" {
		t.Fatalf("got %v %q %v", typ, payload, err)
	}
	typ, payload, err = readFrame(&buf)
	if err != nil || typ != MsgPing || len(payload) != 0 {
		t.Fatalf("got %v %q %v", typ, payload, err)
	}
	if _, _, err = readFrame(&buf); !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF, got %v", err)
	}
}

func TestReadFrameErrors(t *testing.T) {
	big := make([]byte, 6)
	binary.BigEndian.PutUint16(big, uint16(MsgUserState))
	binary.BigEndian.PutUint32(big[2:], MaxPayloadSize+1)
	if _, _, err := readFrame(bytes.NewReader(big)); err == nil {
		t.Fatal("oversized frame accepted")
	}

	truncated := []byte{0, 7, 0, 0, 0, 10, 1, 2}
	if _, _, err := readFrame(bytes.NewReader(truncated)); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("want ErrUnexpectedEOF, got %v", err)
	}
}

func TestMsgTypeString(t *testing.T) {
	if MsgServerConfig.String() != "ServerConfig" || MsgPluginDataTransmission.String() != "PluginDataTransmission" {
		t.Fatal("names out of sync with type numbers")
	}
	if MsgType(99).String() != "MsgType(99)" {
		t.Fatal(MsgType(99).String())
	}
}
