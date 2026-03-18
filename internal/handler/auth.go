package handler

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
)

const sessionCookieName = "postr_session"
const rememberMeDuration = 30 * 24 * time.Hour

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]struct{}
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]struct{})}
}

func (s *sessionStore) create(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = struct{}{}
}

func (s *sessionStore) exists(token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.sessions[token]
	return ok
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type authCheckResponse struct {
	Authenticated bool `json:"authenticated"`
	AuthEnabled   bool `json:"authEnabled"`
}

func (h *Handler) AuthCheck(c *echo.Context) error {
	if !h.config.AuthEnabled {
		return c.JSON(http.StatusOK, authCheckResponse{Authenticated: true, AuthEnabled: false})
	}
	cookie, err := c.Cookie(sessionCookieName)
	authenticated := err == nil && h.sessions.exists(cookie.Value)
	return c.JSON(http.StatusOK, authCheckResponse{Authenticated: authenticated, AuthEnabled: true})
}

func (h *Handler) Login(c *echo.Context) error {
	if !h.config.AuthEnabled {
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	}

	var body struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		RememberMe bool   `json:"rememberMe"`
	}
	if err := c.Bind(&body); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request")
	}

	userMatch := subtle.ConstantTimeCompare([]byte(body.Username), []byte(h.config.AuthUser)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(body.Password), []byte(h.config.AuthPass)) == 1
	if !userMatch || !passMatch {
		return jsonError(c, http.StatusUnauthorized, "invalid credentials")
	}

	token, err := generateToken()
	if err != nil {
		return jsonInternalError(c)
	}
	h.sessions.create(token)

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if body.RememberMe {
		cookie.Expires = time.Now().Add(rememberMeDuration)
	}
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) Logout(c *echo.Context) error {
	cookie, err := c.Cookie(sessionCookieName)
	if err == nil {
		h.sessions.delete(cookie.Value)
	}
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
