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

	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

var safePart = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Archive struct {
	root  string
	level int
	store *storage.Store
}

func New(root string, level int, store *storage.Store) *Archive {
	return &Archive{root: root, level: level, store: store}
}

func (a *Archive) Record(ctx context.Context, meta riotapi.ResponseMeta, body []byte) error {
	if !safePart.MatchString(meta.Region) || !safePart.MatchString(meta.Kind) || !safePart.MatchString(meta.MatchID) {
		return fmt.Errorf("unsafe archive identity region=%q kind=%q match_id=%q", meta.Region, meta.Kind, meta.MatchID)
	}
	rel := filepath.Join(strings.ToUpper(meta.Region), meta.Kind, meta.MatchID+".json.gz")
	path := filepath.Join(a.root, rel)
	digest := sha256.Sum256(body)
	want := hex.EncodeToString(digest[:])

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		got, err := digestGzip(path)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("archive checksum conflict for %s: have %s want %s", rel, got, want)
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if err := writeAtomicGzip(path, body, a.level); err != nil {
		if !os.IsExist(err) {
			return err
		}
		got, digestErr := digestGzip(path)
		if digestErr != nil {
			return digestErr
		}
		if got != want {
			return fmt.Errorf("archive checksum conflict for %s: have %s want %s", rel, got, want)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return a.store.UpsertRawAPIResponse(ctx, storage.RawAPIResponse{
		MatchID: meta.MatchID, Region: strings.ToUpper(meta.Region), Kind: meta.Kind,
		RelativePath: filepath.ToSlash(rel), SHA256: want, RawSize: int64(len(body)), CompressedSize: info.Size(),
	})
}

func writeAtomicGzip(path string, body []byte, level int) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".raw-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o640); err != nil {
		tmp.Close()
		return err
	}
	zw, err := gzip.NewWriterLevel(tmp, level)
	if err != nil {
		tmp.Close()
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
	// Link publishes without replacing an archive concurrently created by
	// another worker. The deferred remove drops the temporary link.
	return os.Link(tmpName, path)
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
