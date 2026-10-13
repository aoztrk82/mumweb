package bridge

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// tokenBucket is a simple, non-concurrent token bucket.
type tokenBucket struct {
	tokens float64
	last   time.Time
}

// take refills at rate tokens/second up to burst and consumes one token if available.
func (b *tokenBucket) take(now time.Time, rate, burst float64) bool {
	if b.last.IsZero() {
		b.tokens = burst
	} else {
		b.tokens = min(burst, b.tokens+now.Sub(b.last).Seconds()*rate)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// ipLimiter limits new connections per client IP.
type ipLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // tokens per second
	burst   float64
}

func newIPLimiter(perMinute int) *ipLimiter {
	return &ipLimiter{
		buckets: map[string]*tokenBucket{},
		rate:    float64(perMinute) / 60,
		burst:   float64(perMinute),
	}
}

func (l *ipLimiter) allow(ip string) bool {
	if l.burst <= 0 {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 4096 {
		// Drop buckets that have fully refilled; they carry no state.
		full := time.Duration(l.burst / l.rate * float64(time.Second))
		for k, b := range l.buckets {
			if now.Sub(b.last) > full {
				delete(l.buckets, k)
			}
		}
	}
	b, ok := l.buckets[ip]
	if !ok {
		b = &tokenBucket{}
		l.buckets[ip] = b
	}
	return b.take(now, l.rate, l.burst)
}

// clientIP returns the remote IP. With trustProxy, the right-most
// X-Forwarded-For entry (the one appended by our reverse proxy) is used.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
