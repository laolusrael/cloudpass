package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

const (
	cSRFCookieName = "cloudpass_session"
	cSRFHeaderName = "X-CSRF-Token"
	cSRFTokenLen   = 32
	cSRFTokenTTL   = 24 * 60 * 60 // 24 hours
	// Rotate the token only once it is past this fraction of its TTL, so
	// concurrent tabs, SSE reconnects, and polling do not invalidate tokens
	// held by in-flight pages. There is no login here and the session cookie
	// is already a random secret, so fixation is not a concern.
	cSRFRotateAfter = 3 * cSRFTokenTTL / 4
)

type csrfTokenEntry struct {
	token   string
	expires int64
}

type csrfStore struct {
	mu     sync.RWMutex
	tokens map[string]*csrfTokenEntry
}

func NewCSRFStore() *csrfStore {
	s := &csrfStore{
		tokens: make(map[string]*csrfTokenEntry),
	}
	go s.cleanupLoop()
	return s
}

func (s *csrfStore) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

func (s *csrfStore) cleanup() {
	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, entry := range s.tokens {
		if now > entry.expires {
			delete(s.tokens, key)
		}
	}
}

func (s *csrfStore) Get(sessionKey string) string {
	now := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.tokens[sessionKey]
	if !ok {
		return ""
	}

	if now > entry.expires {
		delete(s.tokens, sessionKey)
		return ""
	}

	return entry.token
}

// ensureToken returns the live token for a session, minting one only when
// the session has none, it expired, or it is past the rotation threshold.
// Plain reads never rotate, so polling, SSE reconnects, and concurrent tabs
// cannot invalidate tokens held by other pages.
func (s *csrfStore) ensureToken(sessionKey string, generate func() (string, error)) (string, error) {
	now := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.tokens[sessionKey]; ok && now <= entry.expires {
		if now-(entry.expires-cSRFTokenTTL) < cSRFRotateAfter {
			return entry.token, nil
		}
	}

	token, err := generate()
	if err != nil {
		return "", err
	}
	s.tokens[sessionKey] = &csrfTokenEntry{
		token:   token,
		expires: now + cSRFTokenTTL,
	}
	return token, nil
}

func (s *csrfStore) Set(sessionKey, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[sessionKey] = &csrfTokenEntry{
		token:   token,
		expires: time.Now().Unix() + cSRFTokenTTL,
	}
}

func (s *csrfStore) Delete(sessionKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, sessionKey)
}

type csrfMiddleware struct {
	store      *csrfStore
	cookieName string
	headerName string
	tokenLen   int
}

func (m *csrfMiddleware) generateToken() (string, error) {
	b := make([]byte, m.tokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (m *csrfMiddleware) getSessionKey(r *http.Request) (string, error) {
	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func isSafeMethod(method string) bool {
	return method == "GET" || method == "HEAD" || method == "OPTIONS"
}

func NewCSRF(store *csrfStore) echo.MiddlewareFunc {
	m := &csrfMiddleware{
		store:      store,
		cookieName: cSRFCookieName,
		headerName: cSRFHeaderName,
		tokenLen:   cSRFTokenLen,
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			method := req.Method

			// Get or create session
			sessionKey, err := m.getSessionKey(req)
			if err != nil {
				token, tokErr := m.generateToken()
				if tokErr != nil {
					log.Error().Err(tokErr).Msg("failed to generate session token")
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"error": "internal_error",
					})
				}
				sessionKey = token
				m.store.Set(sessionKey, token)

				c.SetCookie(&http.Cookie{
					Name:     cSRFCookieName,
					Value:    sessionKey,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					Secure:   false,
				})
			}

			// Safe methods never invalidate a live token: mint one only when
			// the session has none, it expired, or rotation is due. This keeps
			// polling, SSE reconnects, and concurrent tabs from racing
			// in-flight mutating requests.
			if isSafeMethod(method) {
				if _, genErr := m.store.ensureToken(sessionKey, m.generateToken); genErr != nil {
					log.Error().Err(genErr).Msg("failed to generate CSRF token")
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"error": "internal_error",
					})
				}
			}

			// For mutating methods, validate token before processing
			if !isSafeMethod(method) {
				storedToken := m.store.Get(sessionKey)
				if storedToken == "" {
					return c.JSON(http.StatusForbidden, map[string]string{
						"error":   "csrf_session_expired",
						"message": "Session has expired. Please refresh the page.",
					})
				}

				providedToken := req.Header.Get(cSRFHeaderName)
				if providedToken == "" {
					return c.JSON(http.StatusForbidden, map[string]string{
						"error":   "csrf_token_missing",
						"message": "CSRF token is required",
					})
				}

				if providedToken != storedToken {
					log.Warn().Str("ip", c.RealIP()).Msg("CSRF token validation failed")
					return c.JSON(http.StatusForbidden, map[string]string{
						"error":   "csrf_token_invalid",
						"message": "CSRF token is invalid",
					})
				}
			}

			return next(c)
		}
	}
}

func CSRFTokenHandler(store *csrfStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		cookie, err := req.Cookie(cSRFCookieName)
		if err != nil {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"csrf_token":    nil,
				"needs_session": true,
			})
		}

		token := store.Get(cookie.Value)
		if token == "" {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"csrf_token":    nil,
				"needs_session": true,
			})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"csrf_token": token,
		})
	}
}
