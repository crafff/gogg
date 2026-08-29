package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler/phase3"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase5"
	"github.com/crafff/gogg/apps/worker/internal/rawarchive"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
	"github.com/jackc/pgx/v5"
)

const bundleSchemaVersion = 1

type bundleManifest struct {
	SchemaVersion int            `json:"schema_version"`
	BundleID      string         `json:"bundle_id"`
	CreatedAt     time.Time      `json:"created_at"`
	Objects       []bundleObject `json:"objects"`
}

type bundleObject struct {
	MatchID        string `json:"match_id"`
	Region         string `json:"region"`
	Kind           string `json:"kind"`
	Path           string `json:"path"`
	SHA256         string `json:"sha256"`
	RawSize        int64  `json:"raw_size"`
	CompressedSize int64  `json:"compressed_size"`
}

type bundleMatchMeta struct {
	MatchID      string                  `json:"match_id"`
	Region       string                  `json:"region"`
	Version      string                  `json:"version"`
	AvgTierScore *int                    `json:"avg_tier_score,omitempty"`
	TierCoverage *int16                  `json:"tier_coverage,omitempty"`
	Participants []bundleParticipantMeta `json:"participants,omitempty"`
}

type bundleParticipantMeta struct {
	ParticipantID int     `json:"participant_id"`
	Tier          *string `json:"tier,omitempty"`
	Division      *string `json:"division,omitempty"`
	LP            *int    `json:"lp,omitempty"`
	DeltaHours    *int    `json:"delta_hours,omitempty"`
}

func runBundle(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("bundle requires export or import")
	}
	switch args[0] {
	case "export":
		fs := newFlagSet("bundle export")
		output := fs.String("output", "", "output .tar path")
		retryBatch := fs.String("retry-batch", "", "recreate a completed bundle by ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *output == "" {
			return fmt.Errorf("--output is required")
		}
		return exportBundle(*output, *retryBatch)
	case "import":
		fs := newFlagSet("bundle import")
		input := fs.String("input", "", "input .tar path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *input == "" {
			return fmt.Errorf("--input is required")
		}
		return importBundle(*input)
	default:
		return fmt.Errorf("unknown bundle command %q", args[0])
	}
}

func newFlagSet(name string) *flag.FlagSet { return flag.NewFlagSet(name, flag.ContinueOnError) }

func exportBundle(output, retryBatch string) error {
	ctx, rt, err := boot()
	if err != nil {
		return err
	}
	defer rt.Close()
	if !rt.Cfg.RawArchive.Enabled {
		return errors.New("raw_archive must be enabled")
	}
	var items []storage.RawAPIResponse
	var bundleID string
	createdAt := time.Now().UTC()
	if retryBatch != "" {
		createdAt, items, err = rt.Store.GetRawExportBatch(ctx, retryBatch)
		bundleID = retryBatch
	} else {
		items, err = rt.Store.ListUnexportedRawResponses(ctx)
		if err == nil && len(items) == 0 {
			return errors.New("no unexported raw responses")
		}
		seenIDs := map[string]bool{}
		var matchIDs []string
		for _, item := range items {
			if !seenIDs[item.MatchID] {
				seenIDs[item.MatchID] = true
				matchIDs = append(matchIDs, item.MatchID)
			}
		}
		if err == nil {
			items, err = rt.Store.ListRawResponsesForMatches(ctx, matchIDs)
		}
		if err == nil {
			bundleID, err = newBundleID()
		}
	}
	if err != nil {
		return err
	}
	manifest := bundleManifest{SchemaVersion: bundleSchemaVersion, BundleID: bundleID, CreatedAt: createdAt}
	for _, item := range items {
		manifest.Objects = append(manifest.Objects, bundleObject{MatchID: item.MatchID, Region: item.Region, Kind: item.Kind,
			Path: filepath.ToSlash(filepath.Join("objects", item.RelativePath)), SHA256: item.SHA256,
			RawSize: item.RawSize, CompressedSize: item.CompressedSize})
	}
	metas, err := loadBundleMetadata(ctx, rt.Store, items)
	if err != nil {
		return err
	}
	if err := writeBundleAtomic(output, rt.Cfg.RawArchive.Root, manifest, metas); err != nil {
		return err
	}
	if retryBatch != "" {
		return nil
	}
	return rt.Store.CompleteRawExport(ctx, bundleID, filepath.Base(output), items)
}

func newBundleID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "gogg-" + time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b), nil
}

func loadBundleMetadata(ctx context.Context, store *storage.Store, items []storage.RawAPIResponse) ([]bundleMatchMeta, error) {
	seen := map[string]bool{}
	var ids []string
	for _, item := range items {
		if !seen[item.MatchID] {
			seen[item.MatchID] = true
			ids = append(ids, item.MatchID)
		}
	}
	sort.Strings(ids)
	out := make([]bundleMatchMeta, 0, len(ids))
	for _, id := range ids {
		var m bundleMatchMeta
		err := store.Pool.QueryRow(ctx, `SELECT match_id,region,version,avg_tier_score,tier_coverage FROM matches WHERE match_id=$1`, id).
			Scan(&m.MatchID, &m.Region, &m.Version, &m.AvgTierScore, &m.TierCoverage)
		if err != nil {
			return nil, err
		}
		rows, err := store.Pool.Query(ctx, `SELECT participant_id,tier_at_match,division_at_match,lp_at_match,tier_snapshot_delta_h
			FROM match_participants WHERE match_id=$1 ORDER BY participant_id`, id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var p bundleParticipantMeta
			if err := rows.Scan(&p.ParticipantID, &p.Tier, &p.Division, &p.LP, &p.DeltaHours); err != nil {
				rows.Close()
				return nil, err
			}
			m.Participants = append(m.Participants, p)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		out = append(out, m)
	}
	return out, nil
}

func writeBundleAtomic(output, root string, manifest bundleManifest, metas []bundleMatchMeta) error {
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("output already exists: %s", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(output)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".bundle-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	tw := tar.NewWriter(tmp)
	mb, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		err = writeTarBytes(tw, "manifest.json", append(mb, '\n'))
	}
	if err == nil {
		var b strings.Builder
		enc := json.NewEncoder(&b)
		for _, m := range metas {
			if err = enc.Encode(m); err != nil {
				break
			}
		}
		if err == nil {
			err = writeTarBytes(tw, "metadata.jsonl", []byte(b.String()))
		}
	}
	if err == nil {
		for i, obj := range manifest.Objects {
			src := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(obj.Path, "objects/")))
			if err = writeTarFile(tw, obj.Path, src); err != nil {
				return fmt.Errorf("object %d: %w", i, err)
			}
		}
	}
	if closeErr := tw.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, output)
}

func writeTarBytes(tw *tar.Writer, name string, b []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o640, Size: int64(len(b))}); err != nil {
		return err
	}
	_, err := tw.Write(b)
	return err
}
func writeTarFile(tw *tar.Writer, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o640, Size: st.Size()}); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

func importBundle(input string) error {
	ctx, rt, err := boot()
	if err != nil {
		return err
	}
	defer rt.Close()
	if !rt.Cfg.RawArchive.Enabled {
		return errors.New("raw_archive must be enabled")
	}
	dir, err := makeBundleImportTempDir(rt.Cfg.RawArchive.Root)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err = extractBundle(input, dir); err != nil {
		return err
	}
	manifest, metas, err := readAndValidateBundle(dir)
	if err != nil {
		return err
	}
	exists, err := rt.Store.RawImportExists(ctx, manifest.BundleID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	archive := rawarchive.New(rt.Cfg.RawArchive.Root, rt.Cfg.RawArchive.CompressionLevel, rt.Store)
	metaByID := map[string]bundleMatchMeta{}
	for _, m := range metas {
		metaByID[m.MatchID] = m
	}
	objects := append([]bundleObject(nil), manifest.Objects...)
	sort.SliceStable(objects, func(i, j int) bool { return objects[i].Kind == "match-detail" && objects[j].Kind == "timeline" })
	for _, obj := range objects {
		body, err := readGzip(filepath.Join(dir, filepath.FromSlash(obj.Path)))
		if err != nil {
			return err
		}
		if err = archive.Record(ctx, riotapi.ResponseMeta{Region: obj.Region, Kind: obj.Kind, MatchID: obj.MatchID}, body); err != nil {
			return err
		}
		meta := metaByID[obj.MatchID]
		done, err := responseAlreadyDone(ctx, rt.Store, obj.MatchID, obj.Kind)
		if err != nil {
			return err
		}
		if done {
			continue
		}
		if obj.Kind == "match-detail" {
			if err = rt.Store.UpsertMatchID(ctx, obj.MatchID, obj.Region, meta.Version); err != nil {
				return err
			}
			var dto riotapi.MatchDetailDTO
			if err = json.Unmarshal(body, &dto); err != nil {
				return err
			}
			if err = phase3.IngestMatchDetail(ctx, rt.Store, obj.Region, obj.MatchID, &dto); err != nil {
				return err
			}
		} else {
			var dto riotapi.TimelineDTO
			if err = json.Unmarshal(body, &dto); err != nil {
				return err
			}
			items, skills, snaps := phase5.ExtractTimeline(obj.MatchID, &dto)
			if err = rt.Store.SaveTimeline(ctx, obj.MatchID, items, skills, snaps); err != nil {
				return err
			}
		}
	}
	for _, m := range metas {
		if err = applyBundleMetadata(ctx, rt.Store, m); err != nil {
			return err
		}
	}
	return rt.Store.RecordRawImport(ctx, manifest.BundleID, manifest.CreatedAt, len(manifest.Objects))
}

func makeBundleImportTempDir(archiveRoot string) (string, error) {
	tempRoot := filepath.Join(archiveRoot, ".import-tmp")
	if err := os.MkdirAll(tempRoot, 0o750); err != nil {
		return "", fmt.Errorf("create bundle import temp root: %w", err)
	}
	return os.MkdirTemp(tempRoot, "gogg-bundle-import-*")
}

func extractBundle(input, dir string) error {
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(h.Name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe bundle path %q", h.Name)
		}
		if h.Typeflag != tar.TypeReg {
			return fmt.Errorf("unsupported bundle entry %q", h.Name)
		}
		dst := filepath.Join(dir, clean)
		if err = os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
		if err != nil {
			return err
		}
		_, cpErr := io.Copy(out, io.LimitReader(tr, h.Size+1))
		closeErr := out.Close()
		if cpErr != nil {
			return cpErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

func readAndValidateBundle(dir string) (bundleManifest, []bundleMatchMeta, error) {
	var m bundleManifest
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return m, nil, err
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return m, nil, err
	}
	if m.SchemaVersion != bundleSchemaVersion {
		return m, nil, fmt.Errorf("unsupported bundle schema %d", m.SchemaVersion)
	}
	if m.BundleID == "" {
		return m, nil, errors.New("empty bundle_id")
	}
	for _, o := range m.Objects {
		if o.Kind != "match-detail" && o.Kind != "timeline" {
			return m, nil, fmt.Errorf("invalid response kind %q", o.Kind)
		}
		clean := filepath.Clean(filepath.FromSlash(o.Path))
		if filepath.IsAbs(clean) || clean == "objects" || !strings.HasPrefix(clean, "objects"+string(filepath.Separator)) || filepath.ToSlash(clean) != o.Path {
			return m, nil, fmt.Errorf("invalid object path %q", o.Path)
		}
		path := filepath.Join(dir, clean)
		st, statErr := os.Stat(path)
		if statErr != nil {
			return m, nil, statErr
		}
		if st.Size() != o.CompressedSize {
			return m, nil, fmt.Errorf("compressed size mismatch: %s", o.Path)
		}
		body, err := readGzip(path)
		if err != nil {
			return m, nil, err
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != o.SHA256 {
			return m, nil, fmt.Errorf("checksum mismatch: %s", o.Path)
		}
		if int64(len(body)) != o.RawSize {
			return m, nil, fmt.Errorf("size mismatch: %s", o.Path)
		}
	}
	f, err := os.Open(filepath.Join(dir, "metadata.jsonl"))
	if err != nil {
		return m, nil, err
	}
	defer f.Close()
	var metas []bundleMatchMeta
	dec := json.NewDecoder(f)
	for {
		var x bundleMatchMeta
		if err = dec.Decode(&x); err == io.EOF {
			break
		}
		if err != nil {
			return m, nil, err
		}
		metas = append(metas, x)
	}
	return m, metas, nil
}
func readGzip(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
func responseAlreadyDone(ctx context.Context, s *storage.Store, id, kind string) (bool, error) {
	var done bool
	column := "fetch_status"
	if kind == "timeline" {
		column = "timeline_status"
	}
	err := s.Pool.QueryRow(ctx, "SELECT COALESCE(("+column+"='done'),false) FROM matches WHERE match_id=$1", id).Scan(&done)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return done, err
}
func applyBundleMetadata(ctx context.Context, s *storage.Store, m bundleMatchMeta) error {
	_, err := s.Pool.Exec(ctx, `UPDATE matches SET avg_tier_score=COALESCE(avg_tier_score,$2),tier_coverage=COALESCE(tier_coverage,$3) WHERE match_id=$1`, m.MatchID, m.AvgTierScore, m.TierCoverage)
	if err != nil {
		return err
	}
	for _, p := range m.Participants {
		_, err = s.Pool.Exec(ctx, `UPDATE match_participants SET tier_at_match=COALESCE(tier_at_match,$3),division_at_match=COALESCE(division_at_match,$4),lp_at_match=COALESCE(lp_at_match,$5),tier_snapshot_delta_h=COALESCE(tier_snapshot_delta_h,$6) WHERE match_id=$1 AND participant_id=$2`, m.MatchID, p.ParticipantID, p.Tier, p.Division, p.LP, p.DeltaHours)
		if err != nil {
			return err
		}
	}
	return nil
}
