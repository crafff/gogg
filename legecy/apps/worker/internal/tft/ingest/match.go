package ingest

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/crafff/gogg/packages/riotapi"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const StandardRankedQueueID = 1100

type UnitDefinition struct {
	Purchasable bool
	Cost        int
}

type Catalog map[string]UnitDefinition

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type Result struct {
	MatchID             string
	Eligible            bool
	ParticipantCount    int
	EligibleParticipant int
}

func (s *Store) Match(ctx context.Context, dto *riotapi.TFTMatchDTO, platform, routingRegion string, catalog Catalog, staticRevisionID *int64, targetPatch string) (Result, error) {
	if dto == nil {
		return Result{}, fmt.Errorf("nil TFT match")
	}
	matchID := strings.TrimSpace(dto.Metadata.MatchID)
	if matchID == "" {
		return Result{}, fmt.Errorf("TFT match metadata.match_id is empty")
	}
	info := dto.Info
	eligible, exclusion := matchEligibility(info)
	patch := NormalizePatch(info.GameVersion)
	eligible, exclusion = applyTargetPatchScope(eligible, exclusion, patch, targetPatch)
	gameMS := info.GameDatetime
	if gameMS == 0 {
		gameMS = info.GameCreation
	}
	gameTime := time.UnixMilli(gameMS).UTC()
	result := Result{MatchID: matchID, Eligible: eligible, ParticipantCount: len(info.Participants)}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin TFT match ingest: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := sqlcgen.New(tx)
	if err := q.UpsertTFTMatch(ctx, sqlcgen.UpsertTFTMatchParams{
		MatchID: matchID, DataVersion: optionalString(dto.Metadata.DataVersion),
		Platform: strings.ToUpper(platform), RoutingRegion: strings.ToUpper(routingRegion),
		QueueID: int32(info.QueueID), GameVersion: info.GameVersion, Patch: patch,
		GameDatetime:      pgtype.Timestamptz{Time: gameTime, Valid: true},
		GameLengthSeconds: optionalFloat(info.GameLength), MapID: optionalInt32(info.MapID),
		TftGameType: optionalString(info.TFTGameType), SetCoreName: optionalString(info.TFTSetCoreName),
		SetNumber: optionalInt32(info.TFTSetNumber), EndOfGameResult: optionalString(info.EndOfGameResult),
		ParticipantCount: int16(len(info.Participants)), Eligible: eligible,
		ExclusionReason: optionalString(exclusion), StaticRevisionID: staticRevisionID,
	}); err != nil {
		return Result{}, fmt.Errorf("upsert TFT match: %w", err)
	}
	if err := q.DeleteTFTMatchParticipants(ctx, matchID); err != nil {
		return Result{}, fmt.Errorf("replace TFT participants: %w", err)
	}
	for _, participant := range info.Participants {
		mappedUnits, signature := lineup(participant.Units, catalog)
		abnormal := !eligible || mappedUnits < 5
		if !abnormal {
			result.EligibleParticipant++
		}
		if err := insertParticipant(ctx, q, matchID, participant, mappedUnits, signature, abnormal, catalog); err != nil {
			return Result{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit TFT match ingest: %w", err)
	}
	return result, nil
}

type ingestQueries interface {
	InsertTFTMatchParticipant(context.Context, sqlcgen.InsertTFTMatchParticipantParams) error
	InsertTFTMatchAugment(context.Context, sqlcgen.InsertTFTMatchAugmentParams) error
	InsertTFTMatchTrait(context.Context, sqlcgen.InsertTFTMatchTraitParams) error
	InsertTFTMatchUnit(context.Context, sqlcgen.InsertTFTMatchUnitParams) error
	InsertTFTMatchUnitItem(context.Context, sqlcgen.InsertTFTMatchUnitItemParams) error
}

func insertParticipant(ctx context.Context, q ingestQueries, matchID string, p riotapi.TFTParticipantDTO, mappedUnits int, signature string, abnormal bool, catalog Catalog) error {
	if strings.TrimSpace(p.Puuid) == "" {
		return fmt.Errorf("TFT match %s has participant without puuid", matchID)
	}
	if err := q.InsertTFTMatchParticipant(ctx, sqlcgen.InsertTFTMatchParticipantParams{
		MatchID: matchID, Puuid: p.Puuid, Placement: int16(p.Placement), Level: optionalInt16(p.Level),
		GoldLeft: optionalInt32(p.GoldLeft), LastRound: optionalInt32(p.LastRound),
		PlayersEliminated: optionalInt32(p.PlayersEliminated), TimeEliminatedSeconds: optionalFloat(p.TimeEliminated),
		TotalDamageToPlayers: optionalInt32(p.TotalDamageToPlayers), CompanionContentID: optionalString(p.Companion.ContentID),
		CompanionItemID: optionalInt32(p.Companion.ItemID), CompanionSkinID: optionalInt32(p.Companion.SkinID),
		CompanionSpecies: optionalString(p.Companion.Species), MappedUnitCount: int16(mappedUnits),
		LineupSignature: optionalString(signature), Abnormal: abnormal,
	}); err != nil {
		return fmt.Errorf("insert TFT participant: %w", err)
	}
	for i, augment := range p.Augments {
		if err := q.InsertTFTMatchAugment(ctx, sqlcgen.InsertTFTMatchAugmentParams{MatchID: matchID, Puuid: p.Puuid, Slot: int16(i), AugmentID: augment}); err != nil {
			return fmt.Errorf("insert TFT augment: %w", err)
		}
	}
	for i, trait := range p.Traits {
		if err := q.InsertTFTMatchTrait(ctx, sqlcgen.InsertTFTMatchTraitParams{
			MatchID: matchID, Puuid: p.Puuid, TraitIndex: int16(i), TraitID: trait.Name,
			NumUnits: optionalInt16(trait.NumUnits), Style: optionalInt16(trait.Style),
			TierCurrent: optionalInt16(trait.TierCurrent), TierTotal: optionalInt16(trait.TierTotal),
		}); err != nil {
			return fmt.Errorf("insert TFT trait: %w", err)
		}
	}
	for i, unit := range p.Units {
		definition, mapped := catalog[unit.CharacterID]
		mapped = mapped && definition.Purchasable && definition.Cost > 0
		if err := q.InsertTFTMatchUnit(ctx, sqlcgen.InsertTFTMatchUnitParams{
			MatchID: matchID, Puuid: p.Puuid, UnitIndex: int16(i), CharacterID: unit.CharacterID,
			Name: optionalString(unit.Name), Rarity: optionalInt16(unit.Rarity), Tier: optionalInt16(unit.Tier), Mapped: mapped,
		}); err != nil {
			return fmt.Errorf("insert TFT unit: %w", err)
		}
		for itemSlot, itemID := range unit.ItemNames {
			if err := q.InsertTFTMatchUnitItem(ctx, sqlcgen.InsertTFTMatchUnitItemParams{
				MatchID: matchID, Puuid: p.Puuid, UnitIndex: int16(i), ItemSlot: int16(itemSlot), ItemID: itemID,
			}); err != nil {
				return fmt.Errorf("insert TFT unit item: %w", err)
			}
		}
	}
	return nil
}

func lineup(units []riotapi.TFTUnitDTO, catalog Catalog) (int, string) {
	ids := make([]string, 0, len(units))
	for _, unit := range units {
		definition, ok := catalog[unit.CharacterID]
		if ok && definition.Purchasable && definition.Cost > 0 {
			ids = append(ids, unit.CharacterID)
		}
	}
	sort.Strings(ids)
	return len(ids), strings.Join(ids, "|")
}

func matchEligibility(info riotapi.TFTMatchInfoDTO) (bool, string) {
	if info.QueueID != StandardRankedQueueID {
		return false, "non_standard_queue"
	}
	if len(info.Participants) != 8 {
		return false, "participant_count"
	}
	placements := make(map[int]struct{}, 8)
	for _, participant := range info.Participants {
		if participant.Placement < 1 || participant.Placement > 8 {
			return false, "invalid_placement"
		}
		placements[participant.Placement] = struct{}{}
	}
	if len(placements) != 8 {
		return false, "duplicate_placement"
	}
	return true, ""
}

func applyTargetPatchScope(eligible bool, exclusion, patch, targetPatch string) (bool, string) {
	if patch == "" {
		return false, "invalid_game_version"
	}
	if targetPatch != "" && patch != targetPatch {
		return false, "out_of_scope_patch"
	}
	return eligible, exclusion
}

var versionPattern = regexp.MustCompile(`(?i)(?:version\s+)?(\d+)\.(\d+)`)

func NormalizePatch(version string) string {
	match := versionPattern.FindStringSubmatch(version)
	if len(match) != 3 {
		return ""
	}
	return match[1] + "." + match[2]
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalInt16(value int) *int16       { v := int16(value); return &v }
func optionalInt32(value int) *int32       { v := int32(value); return &v }
func optionalFloat(value float64) *float64 { return &value }

var _ pgx.Tx = (pgx.Tx)(nil)
