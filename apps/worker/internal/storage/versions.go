package storage

import (
	"context"
	"fmt"
	"time"
)

// VersionBoundaries holds the time window for a game patch.
// PatchEnd is nil when the version is the latest (no successor patch yet).
type VersionBoundaries struct {
	PatchStart time.Time
	PatchEnd   *time.Time
}

// GetVersionBoundaries returns the time window for the given version.
// Pass an empty string to use the latest version.
// PatchEnd is nil if no later version exists in the DB.
func (s *Store) GetVersionBoundaries(ctx context.Context, version string) (*VersionBoundaries, error) {
	var patchStart time.Time
	var patchEnd *time.Time

	var err error
	if version == "" {
		err = s.Pool.QueryRow(ctx, `
			SELECT patch_start_at,
			       NULL::timestamptz
			FROM game_versions
			WHERE is_latest = true
			LIMIT 1`).Scan(&patchStart, &patchEnd)
	} else {
		err = s.Pool.QueryRow(ctx, `
			SELECT v.patch_start_at,
			       (SELECT MIN(n.patch_start_at) FROM game_versions n
			        WHERE n.patch_start_at > v.patch_start_at)
			FROM game_versions v
			WHERE v.version = $1`, version).Scan(&patchStart, &patchEnd)
	}
	if err != nil {
		return nil, fmt.Errorf("get version boundaries (version=%q): %w", version, err)
	}
	if patchStart.IsZero() {
		return nil, fmt.Errorf("version %q has no patch_start_at recorded", version)
	}
	return &VersionBoundaries{PatchStart: patchStart, PatchEnd: patchEnd}, nil
}
