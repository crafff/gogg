package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type RawAPIResponse struct {
	MatchID        string
	Region         string
	Kind           string
	RelativePath   string
	SHA256         string
	RawSize        int64
	CompressedSize int64
	CapturedAt     time.Time
}

func (s *Store) UpsertRawAPIResponse(ctx context.Context, r RawAPIResponse) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO raw_api_responses
		  (match_id, region, response_kind, relative_path, sha256, raw_size, compressed_size)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (match_id, response_kind) DO UPDATE SET
		  region=EXCLUDED.region, relative_path=EXCLUDED.relative_path,
		  sha256=EXCLUDED.sha256, raw_size=EXCLUDED.raw_size,
		  compressed_size=EXCLUDED.compressed_size`,
		r.MatchID, r.Region, r.Kind, r.RelativePath, r.SHA256, r.RawSize, r.CompressedSize)
	return err
}

func (s *Store) ListUnexportedRawResponses(ctx context.Context) ([]RawAPIResponse, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.match_id,r.region,r.response_kind,r.relative_path,r.sha256,
		       r.raw_size,r.compressed_size,r.captured_at
		FROM raw_api_responses r
		WHERE NOT EXISTS (
		  SELECT 1 FROM raw_export_batch_items i
		  JOIN raw_export_batches b ON b.id=i.batch_id AND b.status='completed'
		  WHERE i.match_id=r.match_id AND i.response_kind=r.response_kind)
		ORDER BY r.captured_at,r.match_id,r.response_kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RawAPIResponse
	for rows.Next() {
		var r RawAPIResponse
		if err := rows.Scan(&r.MatchID, &r.Region, &r.Kind, &r.RelativePath, &r.SHA256,
			&r.RawSize, &r.CompressedSize, &r.CapturedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListRawResponsesForMatches expands an incremental selection so each bundle
// carries every archived component available for its matches.
func (s *Store) ListRawResponsesForMatches(ctx context.Context, matchIDs []string) ([]RawAPIResponse, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT match_id,region,response_kind,relative_path,sha256,raw_size,compressed_size,captured_at
		FROM raw_api_responses WHERE match_id=ANY($1)
		ORDER BY captured_at,match_id,response_kind`, matchIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RawAPIResponse
	for rows.Next() {
		var r RawAPIResponse
		if err := rows.Scan(&r.MatchID, &r.Region, &r.Kind, &r.RelativePath, &r.SHA256, &r.RawSize, &r.CompressedSize, &r.CapturedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CompleteRawExport(ctx context.Context, bundleID, output string, items []RawAPIResponse) error {
	return s.WithTx(ctx, func(tx pgx.Tx) error {
		var batchID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO raw_export_batches(bundle_id,output_name,status,completed_at)
			VALUES ($1,$2,'completed',now())
			RETURNING id`, bundleID, output).Scan(&batchID); err != nil {
			return err
		}
		for _, item := range items {
			if _, err := tx.Exec(ctx, `
				INSERT INTO raw_export_batch_items(batch_id,match_id,response_kind)
				VALUES ($1,$2,$3)`, batchID, item.MatchID, item.Kind); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) GetRawExportBatch(ctx context.Context, bundleID string) (time.Time, []RawAPIResponse, error) {
	var created time.Time
	if err := s.Pool.QueryRow(ctx, `SELECT created_at FROM raw_export_batches WHERE bundle_id=$1 AND status='completed'`, bundleID).Scan(&created); err != nil {
		return time.Time{}, nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT r.match_id,r.region,r.response_kind,r.relative_path,r.sha256,r.raw_size,r.compressed_size,r.captured_at FROM raw_export_batch_items i JOIN raw_export_batches b ON b.id=i.batch_id JOIN raw_api_responses r USING(match_id,response_kind) WHERE b.bundle_id=$1 ORDER BY r.captured_at,r.match_id,r.response_kind`, bundleID)
	if err != nil {
		return time.Time{}, nil, err
	}
	defer rows.Close()
	var out []RawAPIResponse
	for rows.Next() {
		var r RawAPIResponse
		if err := rows.Scan(&r.MatchID, &r.Region, &r.Kind, &r.RelativePath, &r.SHA256, &r.RawSize, &r.CompressedSize, &r.CapturedAt); err != nil {
			return time.Time{}, nil, err
		}
		out = append(out, r)
	}
	return created, out, rows.Err()
}

func (s *Store) RawImportExists(ctx context.Context, bundleID string) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM raw_import_batches WHERE bundle_id=$1)`, bundleID).Scan(&exists)
	return exists, err
}

func (s *Store) RecordRawImport(ctx context.Context, bundleID string, createdAt time.Time, count int) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO raw_import_batches(bundle_id,source_created_at,object_count)
		VALUES ($1,$2,$3) ON CONFLICT(bundle_id) DO NOTHING`, bundleID, createdAt, count)
	return err
}
