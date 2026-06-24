// Package ratelimit provides a per-client-IP token-bucket rate limiter as Gin
// middleware, with separate limits for public vs authenticated requests.
package ratelimit

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// store holds one token-bucket limiter per client IP for a single tier.
type store struct {
	mu      sync.Mutex
	clients map[string]*client
	limit   rate.Limit
	burst   int
}

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newStore(perMinute int) *store {
	s := &store{
		clients: make(map[string]*client),
		limit:   rate.Every(time.Minute / time.Duration(perMinute)),
		burst:   perMinute, // allow a full minute's worth as burst, then refill
	}
	go s.cleanup()
	return s
}

// get returns (creating if needed) the limiter for an IP and marks it seen.
func (s *store) get(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[ip]
	if !ok {
		c = &client{limiter: rate.NewLimiter(s.limit, s.burst)}
		s.clients[ip] = c
	}
	c.lastSeen = time.Now()
	return c.limiter
}

// cleanup evicts IPs idle for over 5 minutes so the map doesn't grow unbounded.
func (s *store) cleanup() {
	for range time.Tick(3 * time.Minute) {
		s.mu.Lock()
		for ip, c := range s.clients {
			if time.Since(c.lastSeen) > 5*time.Minute {
				delete(s.clients, ip)
			}
		}
		s.mu.Unlock()
	}
}

// Middleware applies tiered per-IP rate limiting. Requests carrying an
// Authorization header use authPerMin; all others use publicPerMin. Health,
// swagger and the long-lived SSE stream (/events) are exempt so monitoring,
// healthchecks and live event streams are never throttled.
func Middleware(publicPerMin, authPerMin int) gin.HandlerFunc {
	public := newStore(publicPerMin)
	authed := newStore(authPerMin)

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/swagger") || strings.HasPrefix(path, "/events") {
			c.Next()
			return
		}

		st := public
		if c.GetHeader("Authorization") != "" {
			st = authed
		}

		if !st.get(c.ClientIP()).Allow() {
			utils.ErrorResponse(c, http.StatusTooManyRequests, "Rate limit exceeded. Please slow down.")
			c.Abort()
			return
		}
		c.Next()
	}
}
