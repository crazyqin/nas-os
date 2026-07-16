package filetimeline

import (
	"context"
	"testing"
)

func TestCommitVersion(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	data := []byte("hello world")
	v, err := ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user1", "initial commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Version != 1 {
		t.Errorf("expected version 1, got %d", v.Version)
	}
	if v.Author != "user1" {
		t.Errorf("expected author user1, got %s", v.Author)
	}
	if v.Checksum == "" {
		t.Error("checksum should not be empty")
	}
}

func TestMultipleVersions(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	// Commit 3 versions
	for i := 0; i < 3; i++ {
		data := []byte("version content")
		_, err := ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user1", "commit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	versions := ft.ListVersions(ctx, "/test/file.txt")
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}

	if versions[2].Version != 3 {
		t.Errorf("expected version 3, got %d", versions[2].Version)
	}
}

func TestCreateSnapshot(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	data := []byte("snapshot content")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user1", "commit")

	snapshot, err := ft.CreateSnapshot(ctx, "/test/file.txt", "before major change")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !snapshot.IsSnapshot {
		t.Error("expected IsSnapshot to be true")
	}
	if snapshot.SnapshotID == "" {
		t.Error("snapshot ID should not be empty")
	}
}

func TestCreateSnapshotNoVersions(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	_, err := ft.CreateSnapshot(ctx, "/nonexistent/file.txt", "snapshot")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestGetTimeline(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	// Create some versions
	data := []byte("content")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user1", "v1")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user1", "v2")
	ft.CreateSnapshot(ctx, "/test/file.txt", "snapshot")

	timeline, err := ft.GetTimeline(ctx, "/test/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if timeline == nil {
		t.Fatal("timeline should not be nil")
	}
}

func TestGetTimelineNoVersions(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	_, err := ft.GetTimeline(ctx, "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestDiffVersions(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	data1 := []byte("version 1")
	data2 := []byte("version 2")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data1)), data1, "user1", "v1")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data2)), data2, "user1", "v2")

	diff, err := ft.DiffVersions(ctx, "/test/file.txt", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff.OldVersion != 1 || diff.NewVersion != 2 {
		t.Errorf("unexpected versions: %d -> %d", diff.OldVersion, diff.NewVersion)
	}
	if len(diff.Changes) == 0 {
		t.Error("expected changes")
	}
}

func TestDiffVersionsNotFound(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	_, err := ft.DiffVersions(ctx, "/nonexistent/file.txt", 1, 2)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRestoreVersion(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	data1 := []byte("version 1")
	data2 := []byte("version 2")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data1)), data1, "user1", "v1")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data2)), data2, "user1", "v2")

	result, err := ft.RestoreVersion(ctx, "/test/file.txt", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Version != 1 {
		t.Errorf("expected restore to version 1, got %d", result.Version)
	}

	// Should have 3 versions now (v1, v2, restore)
	versions := ft.ListVersions(ctx, "/test/file.txt")
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}
}

func TestRestoreVersionNotFound(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	data := []byte("content")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user1", "v1")

	_, err := ft.RestoreVersion(ctx, "/test/file.txt", 999)
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}

func TestGetStats(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	data := []byte("content")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user1", "v1")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user2", "v2")
	ft.CreateSnapshot(ctx, "/test/file.txt", "snapshot")

	stats, err := ft.GetStats(ctx, "/test/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.TotalVersions != 3 { // v1, v2, snapshot
		t.Errorf("expected 3 versions, got %d", stats.TotalVersions)
	}
	if stats.SnapshotCount != 1 {
		t.Errorf("expected 1 snapshot, got %d", stats.SnapshotCount)
	}
}

func TestGetStatsNoFile(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	stats, err := ft.GetStats(ctx, "/nonexistent/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.TotalVersions != 0 {
		t.Errorf("expected 0 versions, got %d", stats.TotalVersions)
	}
}

func TestListVersions(t *testing.T) {
	ft := NewFileTimeline()
	ctx := context.Background()

	data := []byte("content")
	ft.CommitVersion(ctx, "/test/file.txt", int64(len(data)), data, "user1", "v1")

	versions := ft.ListVersions(ctx, "/test/file.txt")
	if len(versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(versions))
	}

	// Nonexistent file should return nil
	versions = ft.ListVersions(ctx, "/nonexistent/file.txt")
	if versions != nil {
		t.Errorf("expected nil, got %v", versions)
	}
}
