//go:build !ci
// +build !ci

package web

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every file the UI build produces on disk must be servable from the
// embedded FS. go:embed directory patterns silently exclude files
// beginning with "_" or "." (unless the all: prefix is used), so a Vite
// chunk named _<hash>.js would otherwise be referenced by the app but
// missing from the binary: blank page in every browser, on every load,
// surviving redeploys and cache clears. This test fails the build if the
// embed directives ever stop covering the build output again.
func TestEmbeddedAssetsComplete(t *testing.T) {
	subFS, err := fs.Sub(staticFiles, "build")
	require.NoError(t, err)

	var onDisk []string
	err = filepath.WalkDir("build", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("build", path)
		if err != nil {
			return err
		}
		onDisk = append(onDisk, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, onDisk, "UI build output missing: run the UI build and copy step first")

	for _, f := range onDisk {
		data, err := fs.ReadFile(subFS, f)
		assert.NoError(t, err, "embedded UI is missing on-disk file %s", f)
		assert.NotEmpty(t, data, "embedded UI file %s is empty", f)
	}
}
