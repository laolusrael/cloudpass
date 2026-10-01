//go:build !ci
// +build !ci

package web

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed build/index.html
//go:embed build/_app
var staticFiles embed.FS

func StaticHandlerWithFallback() echo.HandlerFunc {
	subFS, err := fs.Sub(staticFiles, "build")
	if err != nil {
		panic(err)
	}

	return func(c echo.Context) error {
		path := c.Request().URL.Path

		if path == "/" {
			path = "/index.html"
		}

		cleanPath := path
		if len(cleanPath) > 0 && cleanPath[0] == '/' {
			cleanPath = cleanPath[1:]
		}

		data, err := fs.ReadFile(subFS, cleanPath)
		if err != nil {
			// Bundled assets are never valid app routes: a miss means the
			// browser holds a stale manifest from a previous build. Fail
			// with 404 instead of serving index.html, which browsers
			// reject as a module with a MIME-type error and blank page.
			if strings.HasPrefix(cleanPath, "_app/") {
				return echo.NewHTTPError(404, "asset not found")
			}
			indexData, err := fs.ReadFile(subFS, "index.html")
			if err != nil {
				return echo.NewHTTPError(500, "index.html not found")
			}
			c.Response().Header().Set("Content-Type", "text/html")
			c.Response().Header().Set("Cache-Control", CacheControlNoStore)
			c.Response().Write(indexData)
			return nil
		}

		c.Response().Header().Set("Content-Type", contentTypeForPath(cleanPath))
		if cacheControl := cacheControlForPath(cleanPath); cacheControl != "" {
			c.Response().Header().Set("Cache-Control", cacheControl)
		}
		c.Response().Write(data)
		return nil
	}
}

func init() {
	fmt.Println("Web assets embedded successfully")
}
