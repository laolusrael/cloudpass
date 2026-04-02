//go:build !ci
// +build !ci

package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed build/index.html
//go:embed build/_app
var staticFiles embed.FS

func StaticHandler() echo.HandlerFunc {
	subFS, err := fs.Sub(staticFiles, "build")
	if err != nil {
		panic(err)
	}

	return echo.WrapHandler(http.FileServer(http.FS(subFS)))
}

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
			indexData, err := fs.ReadFile(subFS, "index.html")
			if err != nil {
				return echo.NewHTTPError(500, "index.html not found")
			}
			c.Response().Header().Set("Content-Type", "text/html")
			c.Response().Write(indexData)
			return nil
		}

		contentType := getContentType(cleanPath)
		c.Response().Header().Set("Content-Type", contentType)
		c.Response().Write(data)
		return nil
	}
}

func getContentType(path string) string {
	if len(path) >= 4 {
		ext := path[len(path)-4:]
		switch ext {
		case ".html", "html":
			return "text/html"
		case ".woff", "woff":
			return "font/woff"
		case ".woff2", "woff2":
			return "font/woff2"
		}
	}
	if len(path) >= 3 {
		ext := path[len(path)-3:]
		switch ext {
		case ".js", "js":
			return "application/javascript"
		case ".css", "css":
			return "text/css"
		case ".png", "png":
			return "image/png"
		case ".jpg", "jpg":
			return "image/jpeg"
		case ".svg", "svg":
			return "image/svg+xml"
		}
	}
	return "text/plain"
}

func init() {
	fmt.Println("Web assets embedded successfully")
}
