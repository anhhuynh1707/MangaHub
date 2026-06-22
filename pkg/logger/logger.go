// Package logger configures structured (JSON) logging via the standard
// log/slog package and provides a Gin request-logging middleware.
package logger

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Init installs an slog handler as the default logger and bridges the standard
// library `log` package into it, so existing log.Printf calls across the
// codebase are structured too.
//
//   LOG_LEVEL  = debug|info|warn|error   (default info)
//   LOG_FORMAT = text|json               (default text — readable for local dev;
//                                          set json in production for aggregation)
func Init() {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	var handler slog.Handler
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		// Human-readable key=value with a short HH:MM:SS timestamp for local dev.
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: shortenTime,
		})
	}
	slog.SetDefault(slog.New(handler))

	// Route the std log package (used widely in the codebase) through slog.
	log.SetFlags(0)
	log.SetOutput(slogWriter{})
}

// shortenTime trims the timestamp to HH:MM:SS in text mode for readability.
func shortenTime(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		a.Value = slog.StringValue(a.Value.Time().Format("15:04:05"))
	}
	return a
}

// slogWriter adapts io.Writer (what the std log package writes to) onto slog.
type slogWriter struct{}

func (slogWriter) Write(p []byte) (int, error) {
	slog.Info(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// newRequestID returns a short random hex id for correlating a request's logs.
func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// RequestID returns the request id set by RequestLogger (empty if unset).
func RequestID(c *gin.Context) string { return c.GetString("request_id") }

// RequestLogger is a Gin middleware that emits one structured log line per
// request including request_id, user_id (when authenticated), and latency.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqID := newRequestID()
		c.Set("request_id", reqID)
		c.Writer.Header().Set("X-Request-ID", reqID)

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			slog.String("request_id", reqID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int64("latency_ms", time.Since(start).Milliseconds()),
			slog.String("client_ip", c.ClientIP()),
			slog.Int("bytes", c.Writer.Size()),
		}
		if uid := c.GetString("user_id"); uid != "" {
			attrs = append(attrs, slog.String("user_id", uid))
		}

		switch {
		case status >= 500:
			slog.Error("request", attrs...)
		case status >= 400:
			slog.Warn("request", attrs...)
		default:
			slog.Info("request", attrs...)
		}
	}
}
