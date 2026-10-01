package handlers

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cloudpass/internal/logger"
)

const (
	// artifactTTL is how long staged images/uploads survive without being
	// downloaded or consumed. There is deliberately no background sweeper:
	// SweepArtifacts runs only on lifecycle events (job completion,
	// download, server startup), so an idle server spends zero CPU on it.
	artifactTTL = 24 * time.Hour

	// artifactQuotaBytes caps total staged artifact bytes (0 disables).
	// Enforced reactively at the same trigger points.
	artifactQuotaBytes = 50 * 1024 * 1024 * 1024 // 50 GB
)

// artifactPrefixes are the only basenames SweepArtifacts will ever delete.
// Everything else in the staging directory (including user files) is left
// alone, so a misconfigured staging dir cannot cause data loss elsewhere.
var artifactPrefixes = []string{
	"cloudpass-export-",
	"cloudpass-import-",
	"cloudpass-upload-",
}

func isArtifact(name string) bool {
	for _, prefix := range artifactPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

type artifactEntry struct {
	path    string
	size    int64
	modTime time.Time
}

func listArtifacts(stagingDir string) []artifactEntry {
	dir, err := os.Open(stagingDir)
	if err != nil {
		return nil
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil
	}

	var entries []artifactEntry
	for _, name := range names {
		// Sidecars share the artifact lifecycle.
		base := strings.TrimSuffix(name, ".json")
		if !isArtifact(base) {
			continue
		}
		full := filepath.Join(stagingDir, name)
		info, err := os.Stat(full)
		if err != nil || info.IsDir() {
			continue
		}
		entries = append(entries, artifactEntry{path: full, size: info.Size(), modTime: info.ModTime()})
	}
	return entries
}

func removeArtifact(path string) (int64, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	if err := os.Remove(path); err != nil {
		logger.API.Load().Warn().Err(err).Str("path", path).Msg("failed to remove artifact")
		return 0, false
	}
	return info.Size(), true
}

// SweepArtifacts deletes staged artifacts older than the TTL, then enforces
// the quota oldest-first. Returns removed count and freed bytes. Safe to call
// from any lifecycle event; a no-op (beyond one directory read) when idle.
func SweepArtifacts(stagingDir string) (removed int, freedBytes int64) {
	now := time.Now()
	var total int64
	var entries []artifactEntry

	for _, e := range listArtifacts(stagingDir) {
		total += e.size
		if now.Sub(e.modTime) > artifactTTL {
			if freed, ok := removeArtifact(e.path); ok {
				removed++
				freedBytes += freed
				total -= e.size
			}
		} else {
			entries = append(entries, e)
		}
	}

	if artifactQuotaBytes > 0 && total > artifactQuotaBytes {
		sort.Slice(entries, func(i, j int) bool { return entries[i].modTime.Before(entries[j].modTime) })
		for _, e := range entries {
			if total <= artifactQuotaBytes {
				break
			}
			if freed, ok := removeArtifact(e.path); ok {
				removed++
				freedBytes += freed
				total -= e.size
			}
		}
	}

	if removed > 0 {
		logger.API.Load().Info().Str("dir", stagingDir).Int("removed", removed).Int64("freed_bytes", freedBytes).Msg("artifact sweep completed")
	}
	return removed, freedBytes
}
