package web

import (
	"embed"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed build
var staticFiles embed.FS

func StaticHandler() echo.HandlerFunc {
	return echo.WrapHandler(http.FileServer(http.FS(staticFiles)))
}
