package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (h *Handler) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if !h.config.AuthEnabled {
			return next(c)
		}
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || !h.sessions.exists(cookie.Value) {
			return jsonError(c, http.StatusUnauthorized, "unauthorized")
		}
		return next(c)
	}
}
