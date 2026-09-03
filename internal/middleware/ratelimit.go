package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/utils"
)

// visitorTTL is how long an idle limiter is kept before the sweeper drops it.
// Long enough that a burst is still remembered, short enough that the map does
// not grow without bound.
const (
	visitorTTL   = 10 * time.Minute
	sweepEvery   = 5 * time.Minute
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter keeps one token-bucket limiter per client IP.
//
// In-process and therefore per-instance: behind several replicas the effective
// limit multiplies by the replica count. That is an accepted Phase 1 tradeoff
// — swap in a Redis-backed limiter when the API is scaled out.
type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

func newIPRateLimiter(rps float64, burst int) *ipRateLimiter {
	l := &ipRateLimiter{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
	go l.sweep()
	return l
}

func (l *ipRateLimiter) limiterFor(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// sweep evicts limiters that have gone quiet. It runs for the life of the
// process, which matches the lifetime of the limiter itself.
func (l *ipRateLimiter) sweep() {
	ticker := time.NewTicker(sweepEvery)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-visitorTTL)
		l.mu.Lock()
		for ip, v := range l.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimit throttles requests per client IP. Applied to the auth routes,
// where credential stuffing is the concern.
//
// Client IP comes from Gin's ClientIP, which honours X-Forwarded-For only for
// proxies listed in the engine's TrustedProxies — see router.New.
func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	limiter := newIPRateLimiter(cfg.RPS, cfg.Burst)

	return func(c *gin.Context) {
		if !limiter.limiterFor(c.ClientIP()).Allow() {
			// A fixed hint: the token bucket refills continuously, so there is
			// no exact instant to point at.
			c.Header("Retry-After", strconv.Itoa(int(time.Minute.Seconds())))
			utils.Fail(c, utils.ErrTooManyRequests("Too many requests. Please slow down and try again shortly."))
			return
		}
		c.Next()
	}
}
