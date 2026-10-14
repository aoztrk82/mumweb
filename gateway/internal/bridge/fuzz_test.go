package bridge

import (
	"encoding/json"
	"testing"
)

// The browser is untrusted: no input may panic the gateway.

func FuzzParseAudioFrame(f *testing.F) {
	f.Add([]byte{0x01, 0x00, 1, 0, 0, 0, 0, 0, 0, 0, 0xaa})
	f.Add([]byte{0x01, 0x01, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		if p, err := parseAudioFrame(b); err == nil {
			// A frame the gateway accepts must re-encode without panicking.
			_ = encodeAudioFrame(p)
		}
	})
}

// FuzzClientMessage runs arbitrary JSON through decoding and the admin payload validators.
func FuzzClientMessage(f *testing.F) {
	f.Add(`{"t":"hello","username":"a","cert":{"pem":"x","key":"y"}}`)
	f.Add(`{"t":"adminSetACL","channelId":1,"acl":{"inheritAcls":true,"acls":[{"group":"all","grant":4}],"groups":[{"name":"g","add":[1]}]}}`)
	f.Add(`{"t":"adminSetBanList","bans":[{"address":"100::","mask":128,"hash":"ab"}]}`)
	f.Add(`{"t":"adminUpdateUsers","userUpdates":[{"id":2,"name":"x"},{"id":3,"name":null}]}`)
	f.Fuzz(func(t *testing.T, data string) {
		var m clientMsg
		if json.Unmarshal([]byte(data), &m) != nil {
			return
		}
		s := &session{}
		s.rememberBans(nil)
		s.admin = true
		s.rememberBans(nil)
		_, _ = s.checkBans(m.Bans)
		_, _ = checkUserUpdates(m.UserUpdates)
		if m.ACL != nil {
			_, _ = checkACL(1, *m.ACL)
		}
		_ = cleanTokens(m.Tokens)
	})
}
