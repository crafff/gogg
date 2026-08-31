package tft

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const (
	ObservedPreviewDataKind        = "OBSERVED_RUN_PREVIEW"
	ObservedPreviewAlgorithm       = "exact-observed-board-items-stars-v3"
	DefaultObservedPreviewPlatform = "KR"
	DefaultObservedPreviewSamples  = 20
	DefaultObservedPreviewLimit    = 30
	MaxObservedPreviewLimit        = 30
	ObservedPreviewCacheTTL        = 5 * time.Minute
	ObservedPreviewLoadTimeout     = 30 * time.Second
	observedPreviewCacheEntries    = 32
)

type ObservedFilter struct {
	RunID             int64
	Platform, Locale  string
	MinSamples, Limit int
}

type ObservedStaticSnapshot struct {
	Source, Patch, Revision string
}

type ObservedResult struct {
	DataKind, Platform, Locale, AlgorithmVersion string
	RunID                                        int64
	Platforms, RawGameVersions                   []string
	QueueID, SetNumber                           int
	Patch                                        *string
	CatalogSnapshot, AssetSnapshot               *ObservedStaticSnapshot
	SourceMatches, SourceParticipants            int64
	UsableParticipants, ExactLineups             int64
	WindowStart, WindowEnd                       time.Time
	Items                                        []Lineup
}

type observedBase struct {
	row     sqlcgen.GetTFTObservedLineupPreviewRow
	lineups []rawObservedLineup
}

type observedCacheEntry struct {
	base      observedBase
	details   map[string]rawObservedDetails
	expiresAt time.Time
}

type rawObservedLineup struct {
	ID            string   `json:"id"`
	UnitIDs       []string `json:"unit_ids"`
	SampleSize    int64    `json:"sample_size"`
	LobbyCount    int64    `json:"lobby_count"`
	PickRate      float64  `json:"pick_rate"`
	AvgPlacement  float64  `json:"avg_placement"`
	FirstRate     float64  `json:"first_rate"`
	Top4Rate      float64  `json:"top4_rate"`
	ContestedRate float64  `json:"contested_rate"`
}

type rawObservedDetails struct {
	ID                          string                       `json:"id"`
	UnitItems                   []rawObservedUnitItems       `json:"unit_items"`
	StarLevels                  []rawObservedStarLevel       `json:"star_levels"`
	StarCompositionKnownSamples int64                        `json:"star_composition_known_samples"`
	StarCompositions            []rawObservedStarComposition `json:"star_compositions"`
}

type rawObservedUnitItems struct {
	UnitID             string                `json:"unit_id"`
	Items              []rawObservedItem     `json:"items"`
	CoreRank           *int                  `json:"core_rank"`
	AverageItems       float64               `json:"average_items"`
	ItemInvestmentRate float64               `json:"item_investment_rate"`
	EquippedRate       float64               `json:"equipped_rate"`
	ThreeItemRate      float64               `json:"three_item_rate"`
	KnownStarSamples   int64                 `json:"known_star_samples"`
	UnknownStarSamples int64                 `json:"unknown_star_samples"`
	StarCoverage       float64               `json:"star_coverage"`
	StarDistribution   []rawObservedUnitStar `json:"star_distribution"`
}

type rawObservedItem struct {
	ID    string  `json:"id"`
	Count int64   `json:"count"`
	Rate  float64 `json:"rate"`
}

type rawObservedStarLevel struct {
	TotalStars   int     `json:"total_stars"`
	SampleSize   int64   `json:"sample_size"`
	LobbyCount   int64   `json:"lobby_count"`
	Rate         float64 `json:"rate"`
	AvgPlacement float64 `json:"avg_placement"`
	FirstRate    float64 `json:"first_rate"`
	Top4Rate     float64 `json:"top4_rate"`
}

type rawObservedUnitStar struct {
	Stars        int      `json:"stars"`
	SampleSize   int      `json:"sample_size"`
	Rate         float64  `json:"rate"`
	KnownRate    float64  `json:"known_rate"`
	AvgPlacement *float64 `json:"avg_placement"`
	FirstRate    *float64 `json:"first_rate"`
	Top4Rate     *float64 `json:"top4_rate"`
}

type rawObservedStarComposition struct {
	StarLevels   []int    `json:"star_levels"`
	TotalStars   int      `json:"total_stars"`
	SampleSize   int      `json:"sample_size"`
	Rate         float64  `json:"rate"`
	AvgPlacement *float64 `json:"avg_placement"`
	FirstRate    *float64 `json:"first_rate"`
	Top4Rate     *float64 `json:"top4_rate"`
}

func NormalizeObservedFilter(filter ObservedFilter) ObservedFilter {
	filter.Platform = strings.ToUpper(strings.TrimSpace(filter.Platform))
	if filter.Platform == "" {
		filter.Platform = DefaultObservedPreviewPlatform
	}
	filter.Locale = strings.ToLower(strings.TrimSpace(filter.Locale))
	if filter.Locale == "" {
		filter.Locale = "en_us"
	}
	if filter.MinSamples == 0 {
		filter.MinSamples = DefaultObservedPreviewSamples
	}
	if filter.Limit == 0 {
		filter.Limit = DefaultObservedPreviewLimit
	}
	return filter
}

func ValidateObservedFilter(filter ObservedFilter) error {
	if filter.RunID < 0 {
		return &ValidationError{Field: "runId", Message: "must be positive"}
	}
	if filter.Platform != "GLOBAL" && !supportedTFTPlatform(filter.Platform) {
		return &ValidationError{Field: "platform", Message: "unsupported platform"}
	}
	if filter.Locale != "en_us" && filter.Locale != "zh_cn" {
		return &ValidationError{Field: "locale", Message: "must be en_us or zh_cn"}
	}
	if filter.MinSamples < DefaultObservedPreviewSamples || filter.MinSamples > 100_000 {
		return &ValidationError{Field: "minSamples", Message: "must be 20..100000"}
	}
	if filter.Limit < 1 || filter.Limit > MaxObservedPreviewLimit {
		return &ValidationError{Field: "limit", Message: "must be 1..30"}
	}
	return nil
}

func (s *Service) ObservedLineups(ctx context.Context, filter ObservedFilter) (*ObservedResult, error) {
	filter = NormalizeObservedFilter(filter)
	if err := ValidateObservedFilter(filter); err != nil {
		return nil, err
	}

	run, err := s.observedRun(ctx, filter.RunID, filter.Platform)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve TFT observed preview run: %w", err)
	}
	if run.Status != "completed" && run.Status != "completed_with_errors" {
		return nil, &ValidationError{Field: "runId", Message: "run is not complete"}
	}

	base, err := s.loadObservedBase(ctx, run.ID, filter.Platform)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("aggregate TFT observed lineups: %w", err)
	}
	selected := make([]rawObservedLineup, 0, min(filter.Limit, len(base.lineups)))
	for _, item := range base.lineups {
		if item.SampleSize < int64(filter.MinSamples) {
			continue
		}
		selected = append(selected, item)
		if len(selected) == filter.Limit {
			break
		}
	}
	details := map[string]rawObservedDetails{}
	if len(selected) > 0 {
		selectedSignatures := make([]string, 0, len(selected))
		for _, item := range selected {
			selectedSignatures = append(selectedSignatures, item.ID)
		}
		details, err = s.loadObservedDetails(
			ctx,
			run.ID,
			filter.Platform,
			base.row.CatalogSnapshotID,
			selectedSignatures,
		)
		if err != nil {
			return nil, fmt.Errorf("aggregate TFT observed lineup details: %w", err)
		}
	}

	localized, err := s.q.ListLatestTFTLocalizedStaticObjects(ctx, []string{"unit", "item"}, filter.Locale)
	if err != nil {
		return nil, fmt.Errorf("localize TFT observed lineups: %w", err)
	}
	entities, assets := indexLatestLocalizedEntities(localized)
	row := base.row

	result := &ObservedResult{
		DataKind: ObservedPreviewDataKind, RunID: row.RunID,
		Platform: row.Platform, Platforms: row.Platforms,
		QueueID: int(row.QueueID), Locale: filter.Locale,
		RawGameVersions:  row.RawGameVersions,
		AlgorithmVersion: ObservedPreviewAlgorithm,
		SourceMatches:    row.SourceMatches, SourceParticipants: row.SourceParticipants,
		UsableParticipants: row.UsableParticipants, ExactLineups: row.ExactLineups,
		WindowStart: row.WindowStart.Time, WindowEnd: row.WindowEnd.Time,
		AssetSnapshot: assets,
		CatalogSnapshot: &ObservedStaticSnapshot{
			Source: "ddragon", Patch: row.CatalogPatch, Revision: row.CatalogRevision,
		},
		Items: make([]Lineup, 0, len(selected)),
	}
	if row.SetNumber != nil {
		result.SetNumber = int(*row.SetNumber)
	}
	for _, item := range selected {
		detail := details[item.ID]
		lineup := Lineup{
			ID: observedLineupID(item.ID),
			Metrics: Metrics{
				SampleSize: item.SampleSize, LobbyCount: item.LobbyCount,
				PickRate: item.PickRate, AvgPlacement: item.AvgPlacement,
				FirstRate: item.FirstRate, Top4Rate: item.Top4Rate,
				ContestedRate: item.ContestedRate,
			},
			CommonItems: []Count{}, CommonAugments: []Count{}, CommonTraits: []Count{},
			UnitItems: []UnitItems{}, StarLevels: []StarLevelStrength{},
			StarCompositions: []StarCompositionStrength{},
		}
		unitDetails := make(map[string]rawObservedUnitItems, len(detail.UnitItems))
		for _, unitItems := range detail.UnitItems {
			unitDetails[unitItems.UnitID] = unitItems
		}
		for _, id := range item.UnitIDs {
			unit := localizedEntity(entities, id)
			lineup.CoreUnits = append(lineup.CoreUnits, unit)
			rawUnit := unitDetails[id]
			counts := make([]Count, 0, len(rawUnit.Items))
			for _, equipped := range rawUnit.Items {
				counts = append(counts, Count{
					Entity: localizedEntity(entities, equipped.ID),
					Count:  int(equipped.Count), Rate: equipped.Rate,
				})
			}
			starDistribution := make([]UnitStarStrength, 0, len(rawUnit.StarDistribution))
			for _, level := range rawUnit.StarDistribution {
				starDistribution = append(starDistribution, UnitStarStrength{
					Stars: level.Stars, SampleSize: level.SampleSize,
					Rate: level.Rate, KnownRate: level.KnownRate,
					AvgPlacement: level.AvgPlacement, FirstRate: level.FirstRate,
					Top4Rate: level.Top4Rate,
				})
			}
			lineup.UnitItems = append(lineup.UnitItems, UnitItems{
				Unit: unit, CommonItems: counts, CoreRank: rawUnit.CoreRank,
				AverageItems:       rawUnit.AverageItems,
				ItemInvestmentRate: rawUnit.ItemInvestmentRate,
				EquippedRate:       rawUnit.EquippedRate,
				ThreeItemRate:      rawUnit.ThreeItemRate,
				KnownStarSamples:   rawUnit.KnownStarSamples,
				UnknownStarSamples: rawUnit.UnknownStarSamples,
				StarCoverage:       rawUnit.StarCoverage,
				StarDistribution:   starDistribution,
			})
		}
		for _, strength := range detail.StarLevels {
			lineup.StarLevels = append(lineup.StarLevels, StarLevelStrength{
				TotalStars: strength.TotalStars,
				SampleSize: strength.SampleSize, LobbyCount: strength.LobbyCount,
				Rate: strength.Rate, AvgPlacement: strength.AvgPlacement,
				FirstRate: strength.FirstRate, Top4Rate: strength.Top4Rate,
			})
		}
		lineup.StarCompositionKnownSamples = detail.StarCompositionKnownSamples
		lineup.StarCompositionUnknownSamples = max(
			0,
			item.SampleSize-detail.StarCompositionKnownSamples,
		)
		if item.SampleSize > 0 {
			lineup.StarCompositionCoverage = float64(detail.StarCompositionKnownSamples) / float64(item.SampleSize)
		}
		for _, composition := range detail.StarCompositions {
			levels := make([]StarCount, 0, len(composition.StarLevels))
			for _, stars := range composition.StarLevels {
				if len(levels) > 0 && levels[len(levels)-1].Stars == stars {
					levels[len(levels)-1].UnitCount++
					continue
				}
				levels = append(levels, StarCount{Stars: stars, UnitCount: 1})
			}
			lineup.StarCompositions = append(lineup.StarCompositions, StarCompositionStrength{
				Levels: levels, TotalStars: composition.TotalStars,
				SampleSize: composition.SampleSize, Rate: composition.Rate,
				AvgPlacement: composition.AvgPlacement, FirstRate: composition.FirstRate,
				Top4Rate: composition.Top4Rate,
			})
		}
		result.Items = append(result.Items, lineup)
	}
	return result, nil
}

func (s *Service) loadObservedBase(ctx context.Context, runID int64, platform string) (observedBase, error) {
	key := fmt.Sprintf("%d:%s", runID, platform)
	if cached, ok := s.cachedObservedBase(key); ok {
		return cached, nil
	}
	loaded, err, _ := s.previewSF.Do(key, func() (any, error) {
		if cached, ok := s.cachedObservedBase(key); ok {
			return cached, nil
		}
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ObservedPreviewLoadTimeout)
		defer cancel()
		row, err := s.q.GetTFTObservedLineupPreview(loadCtx, runID, platform)
		if err != nil {
			return observedBase{}, err
		}
		lineups, err := decodeObservedLineups(row.Lineups)
		if err != nil {
			return observedBase{}, fmt.Errorf("decode TFT observed lineups: %w", err)
		}
		row.Lineups = nil
		base := observedBase{row: row, lineups: lineups}
		s.storeObservedBase(key, base)
		return base, nil
	})
	if err != nil {
		return observedBase{}, err
	}
	return loaded.(observedBase), nil
}

func (s *Service) loadObservedDetails(
	ctx context.Context,
	runID int64,
	platform string,
	catalogSnapshotID int64,
	signatures []string,
) (map[string]rawObservedDetails, error) {
	if len(signatures) == 0 {
		return map[string]rawObservedDetails{}, nil
	}
	baseKey := fmt.Sprintf("%d:%s", runID, platform)
	flightKey := fmt.Sprintf("details:%s:%d", baseKey, catalogSnapshotID)
	resolved := make(map[string]rawObservedDetails, len(signatures))
	for {
		for signature, detail := range s.availableObservedDetails(baseKey, catalogSnapshotID, signatures) {
			resolved[signature] = detail
		}
		if selected, ok := selectObservedDetails(resolved, signatures); ok {
			return selected, nil
		}
		unresolved := missingDetailSignatures(resolved, signatures)
		result := s.previewSF.DoChan(flightKey, func() (any, error) {
			missing := s.missingObservedDetails(baseKey, catalogSnapshotID, unresolved)
			if len(missing) == 0 {
				return map[string]rawObservedDetails{}, nil
			}
			loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ObservedPreviewLoadTimeout)
			defer cancel()
			value, err := s.q.GetTFTObservedLineupDetails(loadCtx, sqlcgen.GetTFTObservedLineupDetailsParams{
				RunID: runID, Platform: platform,
				CatalogSnapshotID: catalogSnapshotID,
				Signatures:        missing,
			})
			if err != nil {
				return nil, err
			}
			details, err := decodeObservedDetails(value)
			if err != nil {
				return nil, fmt.Errorf("decode TFT observed lineup details: %w", err)
			}
			bySignature := make(map[string]rawObservedDetails, len(details))
			for _, detail := range details {
				bySignature[detail.ID] = detail
			}
			s.storeObservedDetails(baseKey, catalogSnapshotID, bySignature)
			// Return the decoded rows even when the base cache expires while this
			// query is in flight. The current callers can still complete; a later
			// request will refresh the base instead of causing this one to rescan.
			return bySignature, nil
		})
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case loaded := <-result:
			if loaded.Err != nil {
				return nil, loaded.Err
			}
			for signature, detail := range loaded.Val.(map[string]rawObservedDetails) {
				resolved[signature] = detail
			}
		}
	}
}

func selectObservedDetails(
	details map[string]rawObservedDetails,
	signatures []string,
) (map[string]rawObservedDetails, bool) {
	out := make(map[string]rawObservedDetails, len(signatures))
	for _, signature := range signatures {
		detail, ok := details[signature]
		if !ok {
			return nil, false
		}
		out[signature] = detail
	}
	return out, true
}

func missingDetailSignatures(details map[string]rawObservedDetails, signatures []string) []string {
	missing := make([]string, 0, len(signatures))
	for _, signature := range signatures {
		if _, ok := details[signature]; !ok {
			missing = append(missing, signature)
		}
	}
	return missing
}

func (s *Service) missingObservedDetails(key string, catalogSnapshotID int64, signatures []string) []string {
	s.previewMu.RLock()
	defer s.previewMu.RUnlock()
	entry, ok := s.previewCache[key]
	if !ok || entry.base.row.CatalogSnapshotID != catalogSnapshotID || !s.now().Before(entry.expiresAt) {
		return append([]string(nil), signatures...)
	}
	missing := make([]string, 0, len(signatures))
	for _, signature := range signatures {
		if _, found := entry.details[signature]; !found {
			missing = append(missing, signature)
		}
	}
	return missing
}

func (s *Service) availableObservedDetails(
	key string,
	catalogSnapshotID int64,
	signatures []string,
) map[string]rawObservedDetails {
	s.previewMu.RLock()
	defer s.previewMu.RUnlock()
	entry, ok := s.previewCache[key]
	if !ok || entry.base.row.CatalogSnapshotID != catalogSnapshotID || !s.now().Before(entry.expiresAt) {
		return map[string]rawObservedDetails{}
	}
	out := make(map[string]rawObservedDetails, len(signatures))
	for _, signature := range signatures {
		if detail, found := entry.details[signature]; found {
			out[signature] = detail
		}
	}
	return out
}

func (s *Service) storeObservedDetails(key string, catalogSnapshotID int64, details map[string]rawObservedDetails) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	entry, ok := s.previewCache[key]
	if !ok || entry.base.row.CatalogSnapshotID != catalogSnapshotID || !s.now().Before(entry.expiresAt) {
		return
	}
	if entry.details == nil {
		entry.details = make(map[string]rawObservedDetails, len(details))
	}
	for signature, detail := range details {
		entry.details[signature] = detail
	}
	s.previewCache[key] = entry
}

func (s *Service) cachedObservedBase(key string) (observedBase, bool) {
	s.previewMu.RLock()
	entry, ok := s.previewCache[key]
	s.previewMu.RUnlock()
	if !ok || !s.now().Before(entry.expiresAt) {
		return observedBase{}, false
	}
	return entry.base, true
}

func (s *Service) storeObservedBase(key string, base observedBase) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	now := s.now()
	if len(s.previewCache) >= observedPreviewCacheEntries {
		oldestKey := ""
		var oldest time.Time
		for candidate, entry := range s.previewCache {
			if !now.Before(entry.expiresAt) {
				delete(s.previewCache, candidate)
				continue
			}
			if oldestKey == "" || entry.expiresAt.Before(oldest) {
				oldestKey, oldest = candidate, entry.expiresAt
			}
		}
		if len(s.previewCache) >= observedPreviewCacheEntries && oldestKey != "" {
			delete(s.previewCache, oldestKey)
		}
	}
	s.previewCache[key] = observedCacheEntry{
		base: base, details: make(map[string]rawObservedDetails),
		expiresAt: now.Add(ObservedPreviewCacheTTL),
	}
}

func (s *Service) observedRun(ctx context.Context, runID int64, platform string) (sqlcgen.TftCrawlRun, error) {
	if runID > 0 {
		return s.q.GetTFTRunByID(ctx, runID)
	}
	return s.q.GetLatestCompletedTFTObservedRun(ctx, platform)
}

func decodeObservedLineups(value any) ([]rawObservedLineup, error) {
	var body []byte
	switch typed := value.(type) {
	case []byte:
		body = typed
	case string:
		body = []byte(typed)
	default:
		var err error
		body, err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}
	var out []rawObservedLineup
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []rawObservedLineup{}
	}
	return out, nil
}

func decodeObservedDetails(value any) ([]rawObservedDetails, error) {
	var body []byte
	switch typed := value.(type) {
	case []byte:
		body = typed
	case string:
		body = []byte(typed)
	default:
		var err error
		body, err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}
	var out []rawObservedDetails
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []rawObservedDetails{}
	}
	return out, nil
}

func indexLatestLocalizedEntities(rows []sqlcgen.ListLatestTFTLocalizedStaticObjectsRow) (map[string]Entity, *ObservedStaticSnapshot) {
	out := make(map[string]Entity, len(rows))
	var snapshot *ObservedStaticSnapshot
	for _, row := range rows {
		if snapshot == nil {
			snapshot = &ObservedStaticSnapshot{Source: "cdragon", Patch: row.Patch, Revision: row.Revision}
		}
		entity := Entity{ID: row.ObjectID, IconURL: localTFTIconURL(row.Payload, row.Patch, row.Revision)}
		if row.Name != nil {
			entity.Name = *row.Name
		}
		out[row.ObjectID] = entity
	}
	return out, snapshot
}

func observedLineupID(signature string) string {
	digest := sha256.Sum256([]byte(signature))
	return hex.EncodeToString(digest[:8])
}

func supportedTFTPlatform(platform string) bool {
	switch platform {
	case "NA1", "BR1", "LA1", "LA2", "KR", "JP1", "EUN1", "EUW1", "TR1", "ME1", "RU", "OC1", "SG2", "TW2", "VN2":
		return true
	default:
		return false
	}
}
