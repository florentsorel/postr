package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/florentsorel/postr/internal/config"
	"github.com/labstack/echo/v5"
)

// authSetup returns a handler with auth enabled (user=admin, pass=secret).
func authSetup(t *testing.T) *testSetup {
	t.Helper()
	return newTestSetupWithCfg(t, &config.Config{
		AuthEnabled: true,
		AuthUser:    "admin",
		AuthPass:    "secret",
	}, nil)
}

// loginAndGetCookie performs a successful login and returns the session cookie value.
func loginAndGetCookie(t *testing.T, s *testSetup, rememberMe bool) string {
	t.Helper()
	body := `{"username":"admin","password":"secret"}`
	if rememberMe {
		body = `{"username":"admin","password":"secret","rememberMe":true}`
	}
	rec, c := newCtx(t, http.MethodPost, "/api/auth/login", body)
	if err := s.handler.Login(c); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "postr_session" {
			return cookie.Value
		}
	}
	t.Fatal("no session cookie in response")
	return ""
}

// newCtxWithCookie creates a context with a session cookie pre-set.
func newCtxWithCookie(t *testing.T, method, path, token string) (*httptest.ResponseRecorder, *echo.Context) {
	t.Helper()
	rec, c := newCtx(t, method, path, "")
	c.Request().AddCookie(&http.Cookie{Name: "postr_session", Value: token})
	return rec, c
}

// --- AuthCheck ---

func TestAuthCheck_AuthDisabled(t *testing.T) {
	s := newTestSetup(t, nil)
	rec, c := newCtx(t, http.MethodGet, "/api/auth/check", "")
	if err := s.handler.AuthCheck(c); err != nil {
		t.Fatalf("AuthCheck: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	data := decodeJSON[map[string]bool](t, rec.Body.Bytes())
	if !data["authenticated"] {
		t.Error("expected authenticated=true when auth disabled")
	}
	if data["authEnabled"] {
		t.Error("expected authEnabled=false")
	}
}

func TestAuthCheck_NoCookie(t *testing.T) {
	s := authSetup(t)
	rec, c := newCtx(t, http.MethodGet, "/api/auth/check", "")
	if err := s.handler.AuthCheck(c); err != nil {
		t.Fatalf("AuthCheck: %v", err)
	}
	data := decodeJSON[map[string]bool](t, rec.Body.Bytes())
	if data["authenticated"] {
		t.Error("expected authenticated=false without cookie")
	}
	if !data["authEnabled"] {
		t.Error("expected authEnabled=true")
	}
}

func TestAuthCheck_ValidCookie(t *testing.T) {
	s := authSetup(t)
	token := loginAndGetCookie(t, s, false)

	rec, c := newCtxWithCookie(t, http.MethodGet, "/api/auth/check", token)
	if err := s.handler.AuthCheck(c); err != nil {
		t.Fatalf("AuthCheck: %v", err)
	}
	data := decodeJSON[map[string]bool](t, rec.Body.Bytes())
	if !data["authenticated"] {
		t.Error("expected authenticated=true with valid cookie")
	}
}

func TestAuthCheck_InvalidCookie(t *testing.T) {
	s := authSetup(t)
	rec, c := newCtxWithCookie(t, http.MethodGet, "/api/auth/check", "not-a-valid-token")
	if err := s.handler.AuthCheck(c); err != nil {
		t.Fatalf("AuthCheck: %v", err)
	}
	data := decodeJSON[map[string]bool](t, rec.Body.Bytes())
	if data["authenticated"] {
		t.Error("expected authenticated=false with invalid cookie")
	}
}

// --- Login ---

func TestLogin_AuthDisabled(t *testing.T) {
	s := newTestSetup(t, nil)
	rec, c := newCtx(t, http.MethodPost, "/api/auth/login", `{"username":"anyone","password":"anything"}`)
	if err := s.handler.Login(c); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestLogin_WrongCredentials(t *testing.T) {
	s := authSetup(t)
	rec, c := newCtx(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong"}`)
	if err := s.handler.Login(c); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLogin_ValidCredentials(t *testing.T) {
	s := authSetup(t)
	rec, c := newCtx(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"secret"}`)
	if err := s.handler.Login(c); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == "postr_session" {
			session = c
		}
	}
	if session == nil {
		t.Fatal("expected session cookie in response")
	}
	if !session.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
}

func TestLogin_RememberMe_SetsExpiry(t *testing.T) {
	s := authSetup(t)
	rec, c := newCtx(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"secret","rememberMe":true}`)
	if err := s.handler.Login(c); err != nil {
		t.Fatalf("Login: %v", err)
	}
	cookies := rec.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "postr_session" {
			if cookie.Expires.IsZero() {
				t.Error("expected non-zero expiry for rememberMe cookie")
			}
			return
		}
	}
	t.Fatal("no session cookie")
}

func TestLogin_NoRememberMe_NoExpiry(t *testing.T) {
	s := authSetup(t)
	rec, c := newCtx(t, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"secret","rememberMe":false}`)
	if err := s.handler.Login(c); err != nil {
		t.Fatalf("Login: %v", err)
	}
	cookies := rec.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "postr_session" {
			if !cookie.Expires.IsZero() {
				t.Error("expected zero expiry for session cookie (no rememberMe)")
			}
			return
		}
	}
	t.Fatal("no session cookie")
}

// --- Logout ---

func TestLogout_ClearsCookie(t *testing.T) {
	s := authSetup(t)
	token := loginAndGetCookie(t, s, false)

	rec, c := newCtxWithCookie(t, http.MethodPost, "/api/auth/logout", token)
	if err := s.handler.Logout(c); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "postr_session" {
			if cookie.MaxAge != -1 {
				t.Errorf("expected MaxAge=-1 to clear cookie, got %d", cookie.MaxAge)
			}
			return
		}
	}
	t.Fatal("expected Set-Cookie header to clear session")
}

func TestLogout_SessionInvalidatedAfterLogout(t *testing.T) {
	s := authSetup(t)
	token := loginAndGetCookie(t, s, false)

	_, c := newCtxWithCookie(t, http.MethodPost, "/api/auth/logout", token)
	if err := s.handler.Logout(c); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	// AuthCheck with the old token should now return unauthenticated
	rec, c := newCtxWithCookie(t, http.MethodGet, "/api/auth/check", token)
	if err := s.handler.AuthCheck(c); err != nil {
		t.Fatalf("AuthCheck: %v", err)
	}
	data := decodeJSON[map[string]bool](t, rec.Body.Bytes())
	if data["authenticated"] {
		t.Error("expected authenticated=false after logout")
	}
}

// --- RequireAuth middleware ---

func TestRequireAuth_AuthDisabled_Passes(t *testing.T) {
	s := newTestSetup(t, nil)
	called := false
	next := func(c *echo.Context) error {
		called = true
		return nil
	}
	_, c := newCtx(t, http.MethodGet, "/api/media", "")
	if err := s.handler.RequireAuth(next)(c); err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if !called {
		t.Error("expected next to be called when auth disabled")
	}
}

func TestRequireAuth_NoCookie_Returns401(t *testing.T) {
	s := authSetup(t)
	next := func(c *echo.Context) error { return nil }
	rec, c := newCtx(t, http.MethodGet, "/api/media", "")
	if err := s.handler.RequireAuth(next)(c); err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuth_InvalidCookie_Returns401(t *testing.T) {
	s := authSetup(t)
	next := func(c *echo.Context) error { return nil }
	rec, c := newCtxWithCookie(t, http.MethodGet, "/api/media", "invalid-token")
	if err := s.handler.RequireAuth(next)(c); err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuth_ValidCookie_Passes(t *testing.T) {
	s := authSetup(t)
	token := loginAndGetCookie(t, s, false)

	called := false
	next := func(c *echo.Context) error {
		called = true
		return nil
	}
	_, c := newCtxWithCookie(t, http.MethodGet, "/api/media", token)
	if err := s.handler.RequireAuth(next)(c); err != nil {
		t.Fatalf("RequireAuth: %v", err)
	}
	if !called {
		t.Error("expected next to be called with valid cookie")
	}
}
