package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (h *Handler) GetMedia(c *echo.Context) error {
	return c.JSON(http.StatusOK, []any{})
}
