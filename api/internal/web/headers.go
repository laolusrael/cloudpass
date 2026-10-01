package web

import (
	"path/filepath"
	"strings"
)

// ImmutableAssetsPrefix marks content-hashed files that never change in
// place. SvelteKit emits these under _app/immutable with a hash in every
// filename, so they are safe to cache for a year.
const ImmutableAssetsPrefix = "_app/immutable/"

// Entrypoint files that change in place on every build and must never be
// served stale: index.html references the current hashed chunk manifest,
// and version.json is polled to detect new deployments.
const (
	indexHTMLPath   = "index.html"
	versionJSONPath = "_app/version.json"
)

// CacheControlImmutable is sent for content-hashed assets.
const CacheControlImmutable = "public, max-age=31536000, immutable"

// CacheControlNoStore is sent for entrypoint files so the browser always
// fetches the current asset manifest after a redeploy.
const CacheControlNoStore = "no-store"

func contentTypeForPath(path string) string {
	ext := filepath.Ext(path)
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
	case ".html", "html":
		return "text/html"
	case ".json", "json", ".map", ".webmanifest":
		return "application/json"
	case ".txt", "txt":
		return "text/plain"
	case ".ico", "ico":
		return "image/x-icon"
	case ".woff", "woff":
		return "font/woff"
	case ".woff2", "woff2":
		return "font/woff2"
	}

	return "text/plain"
}

func cacheControlForPath(path string) string {
	cleanPath := strings.TrimPrefix(path, "/")
	if strings.HasPrefix(cleanPath, ImmutableAssetsPrefix) {
		return CacheControlImmutable
	}
	if cleanPath == indexHTMLPath || cleanPath == versionJSONPath {
		return CacheControlNoStore
	}
	return ""
}
