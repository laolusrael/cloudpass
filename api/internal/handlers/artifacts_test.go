package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeArtifact(t *testing.T, dir string, name string, content string, age time.Duration) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	require.NoError(t, os.Chtimes(path, time.Now().Add(-age), time.Now().Add(-age)))
	return path
}

func TestSweepArtifacts_RemovesExpiredManaged(t *testing.T) {
	dir := t.TempDir()

	old := writeArtifact(t, dir, "cloudpass-export-vm-1.img", "old", 25*time.Hour)
	oldSidecar := writeArtifact(t, dir, "cloudpass-export-vm-1.img.json", "{}", 25*time.Hour)
	fresh := writeArtifact(t, dir, "cloudpass-import-abc.img", "fresh", time.Hour)
	foreign := writeArtifact(t, dir, "user-data.img", "keep", 48*time.Hour)

	removed, freed := SweepArtifacts(dir)

	assert.Equal(t, 2, removed)
	assert.Positive(t, freed)
	assert.NoFileExists(t, old)
	assert.NoFileExists(t, oldSidecar)
	assert.FileExists(t, fresh)
	assert.FileExists(t, foreign)
}

func TestSweepArtifacts_IdleIsCheap(t *testing.T) {
	dir := t.TempDir()
	writeArtifact(t, dir, "cloudpass-upload-x", "fresh", time.Hour)

	removed, freed := SweepArtifacts(dir)
	assert.Equal(t, 0, removed)
	assert.Equal(t, int64(0), freed)
}

func TestSweepArtifacts_MissingDir(t *testing.T) {
	removed, freed := SweepArtifacts(filepath.Join(t.TempDir(), "nope"))
	assert.Equal(t, 0, removed)
	assert.Equal(t, int64(0), freed)
}
