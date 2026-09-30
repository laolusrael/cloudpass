package config

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Multipass install modes recognized by DetectMultipassMode.
const (
	MultipassModeSnap    = "snap"
	MultipassModeNative  = "native"
	MultipassModeUnknown = "unknown"
)

// Environment describes the host setup CloudPass adapts to: how multipass
// was installed (snap confinement hides host paths such as /tmp from the
// multipass CLI), which hypervisor driver backs instances (native mounts
// only work on some), and whether the server itself is containerized.
type Environment struct {
	MultipassMode string
	Driver        string
	Containerized bool
}

// Seams for hermetic unit tests.
var (
	detectLookPath  = exec.LookPath
	detectStat      = os.Stat
	detectGetDriver = defaultGetDriver
	detectGetenv    = os.Getenv
	detectHomeDir   = os.UserHomeDir
)

// DetectMultipassMode reports how multipass is installed. A binary resolved
// under /snap/ means snap confinement (private /tmp namespace); anything
// else found on PATH counts as native. CLOUDPASS_MULTIPASS_MODE=snap|native
// overrides detection for exotic setups and tests.
func DetectMultipassMode() string {
	if mode := detectGetenv("CLOUDPASS_MULTIPASS_MODE"); mode == MultipassModeSnap || mode == MultipassModeNative {
		return mode
	}
	bin, err := detectLookPath("multipass")
	if err != nil {
		return MultipassModeUnknown
	}
	// NOTE: check the unresolved path on purpose — /snap/bin entries are
	// symlinks to /usr/bin/snap, so resolving first would hide snap installs.
	if strings.HasPrefix(bin, "/snap/") {
		return MultipassModeSnap
	}
	return MultipassModeNative
}

// defaultGetDriver queries the multipass client setting. Empty means unknown;
// callers must fall back to safe defaults, never fail.
func defaultGetDriver() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "multipass", "get", "local.driver").Output()
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(out)))
}

// DetectContainerized reports whether the server itself runs in a container.
func DetectContainerized() bool {
	if _, err := detectStat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := detectStat("/run/.containerenv"); err == nil {
		return true
	}
	return false
}

// DetectEnvironment probes the host setup once; failures degrade to safe
// defaults (unknown mode/driver) rather than errors.
func DetectEnvironment() Environment {
	return Environment{
		MultipassMode: DetectMultipassMode(),
		Driver:        detectGetDriver(),
		Containerized: DetectContainerized(),
	}
}

// DefaultMountType picks the mount type for the detected driver: native
// mounts only exist on Hyper-V (SMB) and QEMU (9P); everything else (and
// unknown drivers) must use classic SSHFS mounts.
func (e Environment) DefaultMountType() string {
	switch e.Driver {
	case "qemu", "hyper-v", "hyperv":
		return "native"
	default:
		return "classic"
	}
}

// SnapDaemonInvisible reports whether a host source path is likely invisible
// to a snap-confined multipass daemon/CLI (private /tmp namespace), even
// though the server process itself can stat it. Used to fail fast with an
// actionable message instead of a deep multipass error.
func (e Environment) SnapDaemonInvisible(hostPath string) bool {
	if e.MultipassMode != MultipassModeSnap {
		return false
	}
	return hostPath == "/tmp" || strings.HasPrefix(hostPath, "/tmp/") ||
		hostPath == "/var/tmp" || strings.HasPrefix(hostPath, "/var/tmp/")
}

// AutoStagingDir picks an upload staging directory for the install mode:
// snap confinement hides the host /tmp from the multipass CLI, so stage
// under a non-hidden $HOME subdirectory (the snap home interface blocks
// dotfiles). Native installs keep the platform temp dir.
func AutoStagingDir(mode string) string {
	if mode == MultipassModeSnap {
		if home, err := detectHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "cloudpass-staging")
		}
	}
	return os.TempDir()
}
