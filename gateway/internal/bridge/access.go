package bridge

import (
	"crypto/sha256"
	"crypto/subtle"
	"sync"
	"time"
)

// Reject reasons for the web access password (MUMWEB_ACCESS_PASSWORD). They are
// not Mumble reject types; the browser maps them to its own messages.
const (
	rejectAccessRequired    = "AccessPasswordRequired"
	rejectAccessWrong       = "AccessPasswordWrong"
	rejectAccessRateLimited = "AccessRateLimited"

	// Reserved web slots (MUMWEB_RESERVED_SLOTS).
	rejectReservedWrong       = "ReservedPasswordWrong"
	rejectReservedRateLimited = "ReservedRateLimited"
	rejectWebFull             = "WebFull"
)

const (
	accessMaxFailures = 5                // wrong attempts per IP ...
	accessWindow      = 10 * time.Minute // ... within this window lock the IP out
)

// accessFailDelay is the pause before answering a wrong password (shortened in tests).
var accessFailDelay = time.Second

// passwordMatches compares in constant time (hashing first hides the length).
func passwordMatches(got, want string) bool {
	a, b := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// failTracker counts wrong access passwords per client IP.
type failTracker struct {
	mu sync.Mutex
	m  map[string]*failEntry
}

type failEntry struct {
	n     int
	first time.Time
}

func newFailTracker() *failTracker { return &failTracker{m: map[string]*failEntry{}} }

// locked reports whether the IP has used up its attempts in the current window.
func (f *failTracker) locked(ip string, now time.Time) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.m[ip]
	return ok && now.Sub(e.first) < accessWindow && e.n >= accessMaxFailures
}

// fail records a wrong attempt.
func (f *failTracker) fail(ip string, now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.m) > 4096 {
		for k, e := range f.m {
			if now.Sub(e.first) >= accessWindow {
				delete(f.m, k)
			}
		}
	}
	if e, ok := f.m[ip]; ok && now.Sub(e.first) < accessWindow {
		e.n++
		return
	}
	f.m[ip] = &failEntry{n: 1, first: now}
}

// ok clears the record after a correct password.
func (f *failTracker) ok(ip string) {
	f.mu.Lock()
	delete(f.m, ip)
	f.mu.Unlock()
}
