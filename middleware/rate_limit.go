package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"golang_crud/utils"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu        sync.Mutex
	visitors  map[string]*visitor
	limit     rate.Limit
	burst     int
	lastPrune time.Time
}

// RateLimit limits each client IP to requestsPerMinute (with a short burst allowance).
// Use it on login/register to slow down password guessing and signup abuse.
//
// Limits: state is kept in memory, so it is per server instance, and behind a reverse
// proxy every client appears as the proxy's IP unless you configure trusted proxies.
// For several instances, use a shared limiter (Redis) or limit at the proxy/API gateway.
func RateLimit(requestsPerMinute, burst int) gin.HandlerFunc {
	l := &ipRateLimiter{
		visitors:  make(map[string]*visitor),
		limit:     rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:     burst,
		lastPrune: time.Now(),
	}

	return func(c *gin.Context) {
		if !l.allow(c.ClientIP()) {
			c.Header("Retry-After", "60")
			utils.Error(c, utils.NewTooManyRequests("Too many requests. Please try again later."))
			return
		}
		c.Next()
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// Drop clients we haven't seen for a while so the map can't grow forever.
	if now.Sub(l.lastPrune) > time.Minute {
		for key, v := range l.visitors {
			if now.Sub(v.lastSeen) > 10*time.Minute {
				delete(l.visitors, key)
			}
		}
		l.lastPrune = now
	}

	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = now
	return v.limiter.Allow()
}
