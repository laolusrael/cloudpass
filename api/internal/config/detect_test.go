package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withDetectSeams(t *testing.T, lookPath func(string) (string, error), getenv func(string) string, homeDir func() (string, error)) {
	t.Helper()

	oldLookPath, oldGetenv, oldHomeDir := detectLookPath, detectGetenv, detectHomeDir
	detectLookPath, detectGetenv, detectHomeDir = lookPath, getenv, homeDir
	t.Cleanup(func() {
		detectLookPath, detectGetenv, detectHomeDir = oldLookPath, oldGetenv, oldHomeDir
	})
}

func TestDetectMultipassMode(t *testing.T) {
	t.Run("snap binary path", func(t *testing.T) {
		withDetectSeams(t,
			func(string) (string, error) { return "/snap/bin/multipass", nil },
			func(string) string { return "" },
			func() (string, error) { return "/home/u", nil },
		)
		assert.Equal(t, MultipassModeSnap, DetectMultipassMode())
	})

	t.Run("native binary path", func(t *testing.T) {
		withDetectSeams(t,
			func(string) (string, error) { return "/usr/bin/multipass", nil },
			func(string) string { return "" },
			func() (string, error) { return "/home/u", nil },
		)
		assert.Equal(t, MultipassModeNative, DetectMultipassMode())
	})

	t.Run("missing binary", func(t *testing.T) {
		withDetectSeams(t,
			func(string) (string, error) { return "", errors.New("not found") },
			func(string) string { return "" },
			func() (string, error) { return "/home/u", nil },
		)
		assert.Equal(t, MultipassModeUnknown, DetectMultipassMode())
	})

	t.Run("env override wins", func(t *testing.T) {
		withDetectSeams(t,
			func(string) (string, error) { return "", errors.New("not found") },
			func(string) string { return MultipassModeSnap },
			func() (string, error) { return "/home/u", nil },
		)
		assert.Equal(t, MultipassModeSnap, DetectMultipassMode())
	})

	t.Run("invalid env override ignored", func(t *testing.T) {
		withDetectSeams(t,
			func(string) (string, error) { return "C:\\Program Files\\multipass\\multipass.exe", nil },
			func(string) string { return "bogus" },
			func() (string, error) { return "C:\\Users\\u", nil },
		)
		assert.Equal(t, MultipassModeNative, DetectMultipassMode())
	})
}

func TestDefaultMountType(t *testing.T) {
	assert.Equal(t, "native", Environment{Driver: "qemu"}.DefaultMountType())
	assert.Equal(t, "native", Environment{Driver: "hyper-v"}.DefaultMountType())
	assert.Equal(t, "classic", Environment{Driver: "virtualbox"}.DefaultMountType())
	assert.Equal(t, "classic", Environment{Driver: ""}.DefaultMountType())
}

func TestSnapDaemonInvisible(t *testing.T) {
	snap := Environment{MultipassMode: MultipassModeSnap}
	assert.True(t, snap.SnapDaemonInvisible("/tmp"))
	assert.True(t, snap.SnapDaemonInvisible("/tmp/foo"))
	assert.False(t, snap.SnapDaemonInvisible("/home/u/projects"))
	assert.False(t, snap.SnapDaemonInvisible("/tmpness"))

	native := Environment{MultipassMode: MultipassModeNative}
	assert.False(t, native.SnapDaemonInvisible("/tmp/foo"))
}

func TestAutoStagingDir(t *testing.T) {
	home := string(filepath.Separator) + "home" + string(filepath.Separator) + "u"
	withDetectSeams(t,
		func(string) (string, error) { return "", errors.New("not found") },
		func(string) string { return "" },
		func() (string, error) { return home, nil },
	)

	assert.Equal(t, filepath.Join(home, "cloudpass-staging"), AutoStagingDir(MultipassModeSnap))
	assert.Equal(t, os.TempDir(), AutoStagingDir(MultipassModeNative))
	assert.Equal(t, os.TempDir(), AutoStagingDir(MultipassModeUnknown))
}

func TestAutoStagingDir_FallsBackWithoutHome(t *testing.T) {
	withDetectSeams(t,
		func(string) (string, error) { return "", errors.New("not found") },
		func(string) string { return "" },
		func() (string, error) { return "", errors.New("no home") },
	)
	assert.Equal(t, os.TempDir(), AutoStagingDir(MultipassModeSnap))
}

func TestDetectEnvironment_DegradesSafely(t *testing.T) {
	oldGetDriver := detectGetDriver
	detectGetDriver = func() string { return "" }
	t.Cleanup(func() { detectGetDriver = oldGetDriver })

	env := DetectEnvironment()
	require.NotEmpty(t, env.MultipassMode)
	assert.False(t, env.Containerized)
}
