//go:build ci
// +build ci

package web

import (
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
)

var staticFiles fs.FS = nil

func StaticHandler() echo.HandlerFunc {
	return echo.WrapHandler(http.FileServer(http.FS(staticFiles)))
}

func StaticHandlerWithFallback() echo.HandlerFunc {
	return echo.WrapHandler(http.FileServer(http.FS(staticFiles)))
}

func init() {
	println("Web assets: embedded files not available (CI mode)")
}
