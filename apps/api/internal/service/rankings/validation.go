package rankings

import (
	"fmt"
	"math"
	"regexp"
)

const (
	MinQueueID           = 0
	MaxQueueID           = 9999
	MinGames             = 1
	MaxGames             = 20000
	MinPositionThreshold = 0.0
	MaxPositionThreshold = 100.0
	MaxVersionLength     = 16
)

var versionPattern = regexp.MustCompile(`^[0-9]{1,2}\.[0-9]{1,2}(?:\.[0-9]{1,4})?$`)

// ValidationError identifies a client-supplied rankings filter that is
// outside the public API contract.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Msg)
}

// ValidateFilter applies the shared REST/GraphQL rankings contract after
// transport-level whitespace and case normalization.
func ValidateFilter(f Filter) error {
	if f.QueueID < MinQueueID || f.QueueID > MaxQueueID {
		return &ValidationError{Field: "queueId", Msg: "must be between 0 and 9999"}
	}
	if f.MinGames < MinGames || f.MinGames > MaxGames {
		return &ValidationError{Field: "minGames", Msg: "must be between 1 and 20000"}
	}
	if math.IsNaN(f.PositionThreshold) || math.IsInf(f.PositionThreshold, 0) ||
		f.PositionThreshold < MinPositionThreshold || f.PositionThreshold > MaxPositionThreshold {
		return &ValidationError{Field: "positionThreshold", Msg: "must be a finite number between 0 and 100"}
	}
	if len(f.Version) > MaxVersionLength {
		return &ValidationError{Field: "version", Msg: "must be at most 16 characters"}
	}
	if f.Version != "" && f.Version != "latest" && !versionPattern.MatchString(f.Version) {
		return &ValidationError{Field: "version", Msg: "must be latest or a numeric patch such as 16.13"}
	}
	switch f.Region {
	case "", "KR", "NA1":
	default:
		return &ValidationError{Field: "region", Msg: "must be KR or NA1"}
	}
	switch f.TierGroup {
	case "", "challenger", "grandmaster_plus", "grandmaster", "master_plus", "master":
	default:
		return &ValidationError{Field: "tier", Msg: "is not a supported tier group"}
	}
	switch f.Position {
	case "", "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY":
	default:
		return &ValidationError{Field: "position", Msg: "must be TOP, JUNGLE, MIDDLE, BOTTOM, or UTILITY"}
	}
	return nil
}
