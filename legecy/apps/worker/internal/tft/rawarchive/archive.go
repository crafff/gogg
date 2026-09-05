package rawarchive

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/crafff/gogg/packages/riotapi"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

var safePart = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Querier interface {
	UpsertTFTRawObject(context.Context, sqlcgen.UpsertTFTRawObjectParams) error
	InsertTFTRawCapture(context.Context, sqlcgen.InsertTFTRawCaptureParams) (sqlcgen.TftRawCapture, error)
}

type Archive struct {
	root  string
	level int
	q     Querier
}

func New(root string, level int, q Querier) *Archive {
	return &Archive{root: root, level: level, q: q}
}

type runIDKey struct{}

func WithRunID(ctx context.Context, runID int64) context.Context {
	return context.WithValue(ctx, runIDKey{}, runID)
}

func (a *Archive) Record(ctx context.Context, meta riotapi.ResponseMeta, body []byte) error {
	if !safePart.MatchString(meta.Region) || !safePart.MatchString(meta.Kind) {
		return fmt.Errorf("unsafe TFT archive identity platform=%q kind=%q", meta.Region, meta.Kind)
	}
	digest := sha256.Sum256(body)
	sha := hex.EncodeToString(digest[:])
	rel := filepath.Join("TFT", strings.ToUpper(meta.Region), meta.Kind, sha[:2], sha+".json.gz")
	path := filepath.Join(a.root, rel)
	if err := writeContentAddressed(path, body, a.level, sha); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := a.q.UpsertTFTRawObject(ctx, sqlcgen.UpsertTFTRawObjectParams{
		Sha256: sha, RelativePath: filepath.ToSlash(rel), ContentType: "application/json",
		RawSize: int64(len(body)), CompressedSize: info.Size(),
	}); err != nil {
		return fmt.Errorf("upsert TFT raw object: %w", err)
	}
	requestDigest := sha256.Sum256([]byte(meta.RequestURL))
	var runID *int64
	if value, ok := ctx.Value(runIDKey{}).(int64); ok && value > 0 {
		runID = &value
	}
	_, err = a.q.InsertTFTRawCapture(ctx, sqlcgen.InsertTFTRawCaptureParams{
		RunID: runID, Platform: strings.ToUpper(meta.Region), RoutingRegion: routingRegion(meta.Region),
		EndpointKind: meta.Kind, ResourceKey: firstNonEmpty(meta.ResourceKey, meta.MatchID),
		RequestFingerprint: hex.EncodeToString(requestDigest[:]), RequestUrl: meta.RequestURL, ObjectSha256: sha,
	})
	if err != nil {
		return fmt.Errorf("insert TFT raw capture: %w", err)
	}
	return nil
}

func writeContentAddressed(path string, body []byte, level int, wantSHA string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		got, err := digestGzip(path)
		if err != nil {
			return err
		}
		if got != wantSHA {
			return fmt.Errorf("TFT archive checksum conflict for %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tft-raw-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o640); err != nil {
		_ = tmp.Close()
		return err
	}
	zw, err := gzip.NewWriterLevel(tmp, level)
	if err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err = io.Copy(zw, bytes.NewReader(body)); err == nil {
		err = zw.Close()
	} else {
		_ = zw.Close()
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
	if err := os.Link(tmpName, path); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func digestGzip(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	h := sha256.New()
	if _, err := io.Copy(h, zr); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func routingRegion(platform string) string {
	switch strings.ToUpper(platform) {
	case "NA1", "BR1", "LA1", "LA2":
		return "AMERICAS"
	case "KR", "JP1":
		return "ASIA"
	case "EUN1", "EUW1", "TR1", "ME1", "RU":
		return "EUROPE"
	case "OC1", "SG2", "TW2", "VN2", "PH2", "TH2":
		return "SEA"
	default:
		return "UNKNOWN"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "unknown"
}

var _ riotapi.ResponseRecorder = (*Archive)(nil)
