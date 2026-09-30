package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCSRFEcho() (*echo.Echo, *csrfStore) {
	e := echo.New()
	store := NewCSRFStore()
	e.Use(NewCSRF(store))
	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.POST("/mut", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/csrf/token", CSRFTokenHandler(store))
	return e, store
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == cSRFCookieName {
			return c
		}
	}
	return nil
}

func getWithCookie(t *testing.T, e *echo.Echo, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func tokenFor(t *testing.T, e *echo.Echo, cookie *http.Cookie) string {
	t.Helper()

	rec := getWithCookie(t, e, "/csrf/token", cookie)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "csrf_token")
	return rec.Body.String()
}

func TestCSRF_TokenStableAcrossReads(t *testing.T) {
	e, _ := setupCSRFEcho()

	first := getWithCookie(t, e, "/ping", nil)
	require.Equal(t, http.StatusOK, first.Code)
	cookie := sessionCookie(first)
	require.NotNil(t, cookie, "session cookie should be set")

	before := tokenFor(t, e, cookie)
	for i := 0; i < 10; i++ {
		rec := getWithCookie(t, e, "/ping", cookie)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	after := tokenFor(t, e, cookie)
	assert.Equal(t, before, after, "plain GETs must not rotate the token")
}

func TestCSRF_MutatingValidation(t *testing.T) {
	e, _ := setupCSRFEcho()

	first := getWithCookie(t, e, "/ping", nil)
	cookie := sessionCookie(first)
	require.NotNil(t, cookie)

	// Extract the raw token to send as header.
	var tokenBody map[string]string
	require.NoError(t, json.Unmarshal(getWithCookie(t, e, "/csrf/token", cookie).Body.Bytes(), &tokenBody))
	token, ok := tokenBody["csrf_token"]
	require.True(t, ok && token != "", "expected a csrf_token in %v", tokenBody)

	post := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mut", nil)
		req.AddCookie(cookie)
		if token != "" {
			req.Header.Set(cSRFHeaderName, token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	assert.Equal(t, http.StatusOK, post(token).Code)
	assert.Equal(t, http.StatusForbidden, post("bogus").Code)
	assert.Contains(t, post("bogus").Body.String(), "csrf_token_invalid")
	assert.Equal(t, http.StatusForbidden, post("").Code)
	assert.Contains(t, post("").Body.String(), "csrf_token_missing")
}

func TestCSRF_RotatesOnlyNearExpiry(t *testing.T) {
	e, store := setupCSRFEcho()

	first := getWithCookie(t, e, "/ping", nil)
	cookie := sessionCookie(first)
	require.NotNil(t, cookie)

	before := tokenFor(t, e, cookie)

	// Age the token past the rotation threshold but before expiry.
	store.mu.Lock()
	store.tokens[cookie.Value].expires = time.Now().Unix() + 10
	store.mu.Unlock()

	rec := getWithCookie(t, e, "/ping", cookie)
	require.Equal(t, http.StatusOK, rec.Code)
	after := tokenFor(t, e, cookie)
	assert.NotEqual(t, before, after, "aged token should rotate on read")
}

func TestCSRF_ExpiredSessionRejected(t *testing.T) {
	e, store := setupCSRFEcho()

	first := getWithCookie(t, e, "/ping", nil)
	cookie := sessionCookie(first)
	require.NotNil(t, cookie)

	store.mu.Lock()
	store.tokens[cookie.Value].expires = time.Now().Unix() - 1
	store.mu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/mut", nil)
	req.AddCookie(cookie)
	req.Header.Set(cSRFHeaderName, "stale")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "csrf_session_expired")
}
