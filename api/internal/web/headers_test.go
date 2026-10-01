package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentTypeForPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"_app/immutable/chunks/B9nDytE-.js", "application/javascript"},
		{"_app/immutable/assets/0.BaY2y-YA.css", "text/css"},
		{"favicon.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"icon.svg", "image/svg+xml"},
		{"index.html", "text/html"},
		{"_app/version.json", "application/json"},
		{"_app/immutable/chunks/B9nDytE-.js.map", "application/json"},
		{"manifest.webmanifest", "application/json"},
		{"robots.txt", "text/plain"},
		{"favicon.ico", "image/x-icon"},
		{"font.woff", "font/woff"},
		{"font.woff2", "font/woff2"},
		{"no-extension", "text/plain"},
		{"unknown.zzz", "text/plain"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.want, contentTypeForPath(tc.path))
		})
	}
}

func TestCacheControlForPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// Content-hashed assets: cache for a year.
		{"_app/immutable/chunks/B9nDytE-.js", CacheControlImmutable},
		{"_app/immutable/assets/0.BaY2y-YA.css", CacheControlImmutable},
		{"_app/immutable/nodes/0.B17eey-l.js", CacheControlImmutable},
		{"/_app/immutable/chunks/B9nDytE-.js", CacheControlImmutable},
		// Entrypoints that change in place: never serve stale.
		{"index.html", CacheControlNoStore},
		{"/index.html", CacheControlNoStore},
		{"_app/version.json", CacheControlNoStore},
		{"_app/env.js", ""},
		// App routes and misc files: no opinion.
		{"/", ""},
		{"instances/web-1", ""},
		{"favicon.png", ""},
		{"favicon.ico", ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.want, cacheControlForPath(tc.path))
		})
	}
}

func TestCacheControlConstants(t *testing.T) {
	assert.Contains(t, CacheControlImmutable, "immutable")
	assert.Contains(t, CacheControlImmutable, "max-age=31536000")
	assert.Equal(t, "no-store", CacheControlNoStore)
}
