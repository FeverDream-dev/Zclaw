package api

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// CorsMiddleware enables CORS for local development.
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RecoveryMiddleware recovers from panics and returns 500.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("{\"error\":\"internal_server_error\"}"))
				log.Printf("panic recovered: %v", rec)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs basic request metrics.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(lrw, r)
		dur := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lrw.status, dur)
	})
}

// RateLimiter is a simple token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	capacity int
	tokens   int
	refill   time.Duration
	last     time.Time
}

func NewRateLimiter(rate int, capacity int) *RateLimiter {
	if capacity <= 0 {
		capacity = 1
	}
	rl := &RateLimiter{capacity: capacity, tokens: capacity, refill: time.Second / time.Duration(rate), last: time.Now()}
	return rl
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	// Refill tokens based on elapsed time
	elapsed := now.Sub(r.last)
	if elapsed > 0 {
		refillTokens := int(elapsed / r.refill)
		if refillTokens > 0 {
			r.tokens += refillTokens
			if r.tokens > r.capacity {
				r.tokens = r.capacity
			}
			r.last = now
		}
	}
	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}

func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rl != nil && !rl.Allow() {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
