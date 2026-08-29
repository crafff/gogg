package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBundleRoundTripValidation(t *testing.T) {
	root := t.TempDir()
	body := []byte(`{"metadata":{"matchId":"KR_1"},"info":{"participants":[]}}`)
	rel := filepath.Join("KR", "match-detail", "KR_1.json.gz")
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	if _, err = zw.Write(body); err == nil {
		err = zw.Close()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	m := bundleManifest{SchemaVersion: bundleSchemaVersion, BundleID: "gogg-test", CreatedAt: time.Now().UTC(), Objects: []bundleObject{{MatchID: "KR_1", Region: "KR", Kind: "match-detail", Path: "objects/" + filepath.ToSlash(rel), SHA256: hex.EncodeToString(sum[:]), RawSize: int64(len(body)), CompressedSize: st.Size()}}}
	bundlePath := filepath.Join(t.TempDir(), "bundle.tar")
	if err = writeBundleAtomic(bundlePath, root, m, []bundleMatchMeta{{MatchID: "KR_1", Region: "KR", Version: "16.13"}}); err != nil {
		t.Fatal(err)
	}
	extracted := t.TempDir()
	if err = extractBundle(bundlePath, extracted); err != nil {
		t.Fatal(err)
	}
	got, metas, err := readAndValidateBundle(extracted)
	if err != nil {
		t.Fatal(err)
	}
	if got.BundleID != "gogg-test" || len(got.Objects) != 1 || len(metas) != 1 {
		t.Fatalf("unexpected bundle: %+v metas=%+v", got, metas)
	}
}

func TestBundleRejectsManifestTraversal(t *testing.T) {
	dir := t.TempDir()
	body := []byte("{}")
	sum := sha256.Sum256(body)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), mustJSON(t, bundleManifest{SchemaVersion: bundleSchemaVersion, BundleID: "bad", CreatedAt: time.Now(), Objects: []bundleObject{{MatchID: "KR_1", Region: "KR", Kind: "timeline", Path: "../outside.json.gz", SHA256: hex.EncodeToString(sum[:]), RawSize: 2}}}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.jsonl"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readAndValidateBundle(dir); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestBundleImportTempDirLivesUnderArchiveRoot(t *testing.T) {
	root := t.TempDir()
	dir, err := makeBundleImportTempDir(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	wantParent := filepath.Join(root, ".import-tmp")
	if filepath.Dir(dir) != wantParent {
		t.Fatalf("temp parent = %q, want %q", filepath.Dir(dir), wantParent)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
