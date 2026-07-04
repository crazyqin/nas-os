package smartfolders

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, root, rel string, data []byte, mod time.Time) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClassify(t *testing.T) {
	tests := map[string]FileClass{
		"family.HEIC": ClassPhoto,
		"movie.mkv":   ClassVideo,
		"song.flac":   ClassAudio,
		"paper.pdf":   ClassDocument,
		"backup.7z":   ClassArchive,
		"main.go":     ClassCode,
		"blob.bin":    ClassOther,
	}
	for name, want := range tests {
		if got := Classify(name); got != want {
			t.Fatalf("Classify(%q)=%q, want %q", name, got, want)
		}
	}
}

func TestListFiltersAndSorts(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	old := now.Add(-30 * 24 * time.Hour)
	newer := now.Add(-1 * time.Hour)
	writeFile(t, root, "photos/old.jpg", []byte("old-photo"), old)
	writeFile(t, root, "photos/new.png", []byte("new-photo"), newer)
	writeFile(t, root, "docs/note.txt", []byte("doc"), newer)

	engine, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	engine.now = func() time.Time { return now }

	res, err := engine.List("", Rule{Classes: []FileClass{ClassPhoto}, ModifiedIn: 7 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if res.Scanned != 3 || res.Matched != 1 || len(res.Items) != 1 {
		t.Fatalf("unexpected result: scanned=%d matched=%d items=%d", res.Scanned, res.Matched, len(res.Items))
	}
	if res.Items[0].Name != "new.png" || res.Items[0].Class != ClassPhoto {
		t.Fatalf("unexpected item: %+v", res.Items[0])
	}
	if res.Summary.TotalSize != int64(len("new-photo")) || res.Summary.ByClass[ClassPhoto] != 1 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
}

func TestListLimitAndExtensionFilter(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeFile(t, root, "a/report.pdf", []byte("one"), now)
	writeFile(t, root, "b/manual.pdf", []byte("two"), now.Add(time.Minute))
	writeFile(t, root, "c/photo.jpg", []byte("img"), now.Add(2*time.Minute))

	engine, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := engine.List("", Rule{Extensions: []string{".pdf"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated || res.Matched != 2 || len(res.Items) != 1 {
		t.Fatalf("limit not enforced as expected: %+v", res)
	}
	if res.Items[0].Extension != "pdf" {
		t.Fatalf("wrong extension: %+v", res.Items[0])
	}
}

func TestSummaryCountsAllMatchesWhenLimited(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeFile(t, root, "a/report.pdf", []byte("one"), now)
	writeFile(t, root, "b/manual.pdf", []byte("two"), now.Add(time.Minute))

	engine, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := engine.List("", Rule{Extensions: []string{"pdf"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 2 || res.Summary.ByClass[ClassDocument] != 2 || res.Summary.TotalSize != 6 {
		t.Fatalf("summary should include all matches before truncation: %+v", res)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	root := t.TempDir()
	engine, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.List("../", Rule{}); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestSymlinkTraversalRejected(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, outside, "secret.txt", []byte("secret"), time.Now())
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	engine, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.List("link", Rule{}); err == nil {
		t.Fatal("expected symlink traversal to be rejected")
	}
}

func TestBuiltInRules(t *testing.T) {
	rules := BuiltInRules()
	if len(rules) < 5 {
		t.Fatalf("expected starter rules, got %d", len(rules))
	}
	if rules[0].Name != "recent" || rules[0].ModifiedIn <= 0 {
		t.Fatalf("unexpected recent rule: %+v", rules[0])
	}
}
