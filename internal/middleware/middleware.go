package middleware

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"sftp-gui/internal/config"
	"sftp-gui/internal/models"
	"sftp-gui/internal/services"
)

// contextKey is used for context keys to avoid collisions
type contextKey string

const (
	SessionIDKey contextKey = "session_id"
	SessionKey   contextKey = "session"
)

// Middleware holds middleware dependencies
type Middleware struct {
	sessionService *services.SessionService
	config         *config.Config
}

// New creates a new middleware instance
func New(sessionService *services.SessionService, cfg *config.Config) *Middleware {
	return &Middleware{
		sessionService: sessionService,
		config:         cfg,
	}
}

// Logger logs HTTP requests
func (m *Middleware) Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures status code
		lrw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		log.Printf("[%s] %s %s %d %v %s",
			r.Method,
			r.RemoteAddr,
			r.URL.Path,
			lrw.statusCode,
			duration,
			r.UserAgent(),
		)
	})
}

// Recovery recovers from panics and returns a 500 error
func (m *Middleware) Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v\n%s", err, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS handles Cross-Origin Resource Sharing
func (m *Middleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Security.CORSEnabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range m.config.Security.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SessionAuth validates session authentication
func (m *Middleware) SessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get session ID from cookie
		cookie, err := r.Cookie(m.config.Security.SessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		sessionID := cookie.Value
		if sessionID == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		// Validate session
		session, err := m.sessionService.GetSession(sessionID)
		if err != nil {
			// Clear invalid cookie
			http.SetCookie(w, &http.Cookie{
				Name:     m.config.Security.SessionCookieName,
				Value:    "",
				Path:     "/",
				Expires:  time.Unix(0, 0),
				HttpOnly: true,
				Secure:   m.config.Security.SessionCookieSecure,
			})
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		// Add session to context
		ctx := context.WithValue(r.Context(), SessionIDKey, sessionID)
		ctx = context.WithValue(ctx, SessionKey, session)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SecurityHeaders adds security headers
func (m *Middleware) SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// XSS protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Content Security Policy
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://unpkg.com; " +
			"style-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://unpkg.com; " +
			"font-src 'self' https://fonts.gstatic.com; " +
			"img-src 'self' data:; " +
			"connect-src 'self'"
		w.Header().Set("Content-Security-Policy", csp)

		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// RateLimit implements basic rate limiting (simplified version)
func (m *Middleware) RateLimit(next http.Handler) http.Handler {
	// Note: This is a simplified rate limiter
	// In production, consider using a more sophisticated solution like redis-based rate limiting
	clients := make(map[string][]time.Time)
	var mutex sync.RWMutex

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)
		now := time.Now()
		windowSize := time.Minute
		maxRequests := 100 // requests per minute

		mutex.Lock()

		// Clean old entries
		if requests, exists := clients[clientIP]; exists {
			var validRequests []time.Time
			for _, reqTime := range requests {
				if now.Sub(reqTime) < windowSize {
					validRequests = append(validRequests, reqTime)
				}
			}
			clients[clientIP] = validRequests
		}

		// Check rate limit
		if len(clients[clientIP]) >= maxRequests {
			mutex.Unlock()
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Add current request
		clients[clientIP] = append(clients[clientIP], now)
		mutex.Unlock()

		next.ServeHTTP(w, r)
	})
}

// GetSessionFromContext extracts session from request context
func GetSessionFromContext(ctx context.Context) (*models.Session, bool) {
	session, ok := ctx.Value(SessionKey).(*models.Session)
	return session, ok
}

// GetSessionIDFromContext extracts session ID from request context
func GetSessionIDFromContext(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(SessionIDKey).(string)
	return sessionID, ok
}

// loggingResponseWriter captures the status code for logging
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}
