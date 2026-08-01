package rawarchive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicGzipRoundTripAndNoReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "KR_1.json.gz")
	body := []byte(`{"metadata":{"matchId":"KR_1"}}`)
	if err := writeAtomicGzip(path, body, 6); err != nil {
		t.Fatal(err)
	}
	got, err := digestGzip(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ee8d599a1f6ba2bfe0d7042b46c3ca6c4966e654936c0b89688bf89a505ad976" {
		t.Fatalf("digest = %s", got)
	}
	if err := writeAtomicGzip(path, []byte("different"), 6); !os.IsExist(err) {
		t.Fatalf("second write error = %v, want exists", err)
	}
	got2, err := digestGzip(path)
	if err != nil {
		t.Fatal(err)
	}
	if got2 != got {
		t.Fatalf("archive was replaced: %s -> %s", got, got2)
	}
}

func TestSafeArchiveIdentity(t *testing.T) {
	for _, value := range []string{"KR", "match-detail", "KR_123-abc"} {
		if !safePart.MatchString(value) {
			t.Fatalf("expected safe: %q", value)
		}
	}
	for _, value := range []string{"../KR", "timeline/x", "", "KR 1"} {
		if safePart.MatchString(value) {
			t.Fatalf("expected unsafe: %q", value)
		}
	}
}
