package snapdiffviz

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateDiff_NoChanges(t *testing.T) {
	before := map[string]int64{}
	after := map[string]int64{}

	signal := GenerateDiff("snap-001", "snap-002", "tank/data", before, after)

	if signal.AddedFiles != 0 || signal.ModifiedFiles != 0 || signal.DeletedFiles != 0 {
		t.Errorf("expected zero changes, got added=%d modified=%d deleted=%d",
			signal.AddedFiles, signal.ModifiedFiles, signal.DeletedFiles)
	}

	if signal.AddedBytes != 0 || signal.ModifiedBytes != 0 || signal.DeletedBytes != 0 {
		t.Errorf("expected zero bytes changed, got added=%d modified=%d deleted=%d",
			signal.AddedBytes, signal.ModifiedBytes, signal.DeletedBytes)
	}

	if !strings.Contains(signal.DiffTree, "(no changes)") {
		t.Errorf("expected diff tree to indicate no changes, got: %s", signal.DiffTree)
	}

	if signal.SnapshotBefore != "snap-001" || signal.SnapshotAfter != "snap-002" {
		t.Errorf("unexpected snapshot names: before=%s after=%s", signal.SnapshotBefore, signal.SnapshotAfter)
	}

	if signal.Dataset != "tank/data" {
		t.Errorf("expected dataset tank/data, got %s", signal.Dataset)
	}

	if signal.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestGenerateDiff_AllAdded(t *testing.T) {
	before := map[string]int64{}
	after := map[string]int64{
		"file1.txt": 100,
		"file2.txt": 200,
		"file3.txt": 300,
	}

	signal := GenerateDiff("snap-001", "snap-002", "tank/data", before, after)

	if signal.AddedFiles != 3 {
		t.Errorf("expected 3 added files, got %d", signal.AddedFiles)
	}

	if signal.AddedBytes != 600 {
		t.Errorf("expected 600 added bytes, got %d", signal.AddedBytes)
	}

	if signal.DeletedFiles != 0 || signal.ModifiedFiles != 0 {
		t.Errorf("expected 0 deleted and 0 modified, got deleted=%d modified=%d",
			signal.DeletedFiles, signal.ModifiedFiles)
	}

	if signal.ChangesByType["added"] != 3 {
		t.Errorf("expected 3 added in ChangesByType, got %d", signal.ChangesByType["added"])
	}

	// Verify diff tree contains + symbol for added files
	if !strings.Contains(signal.DiffTree, "+") {
		t.Errorf("expected diff tree to contain '+' for added files, got: %s", signal.DiffTree)
	}
}

func TestGenerateDiff_AllDeleted(t *testing.T) {
	before := map[string]int64{
		"file1.txt": 100,
		"file2.txt": 200,
		"file3.txt": 300,
		"file4.txt": 400,
	}
	after := map[string]int64{}

	signal := GenerateDiff("snap-001", "snap-002", "tank/data", before, after)

	if signal.DeletedFiles != 4 {
		t.Errorf("expected 4 deleted files, got %d", signal.DeletedFiles)
	}

	if signal.DeletedBytes != 1000 {
		t.Errorf("expected 1000 deleted bytes, got %d", signal.DeletedBytes)
	}

	if signal.AddedFiles != 0 || signal.ModifiedFiles != 0 {
		t.Errorf("expected 0 added and 0 modified, got added=%d modified=%d",
			signal.AddedFiles, signal.ModifiedFiles)
	}

	if signal.ChangesByType["deleted"] != 4 {
		t.Errorf("expected 4 deleted in ChangesByType, got %d", signal.ChangesByType["deleted"])
	}

	// Verify diff tree contains − symbol for deleted files
	if !strings.Contains(signal.DiffTree, "−") {
		t.Errorf("expected diff tree to contain '−' for deleted files, got: %s", signal.DiffTree)
	}
}

func TestGenerateDiff_ModifiedFiles(t *testing.T) {
	before := map[string]int64{
		"file1.txt": 100,
		"file2.txt": 200,
	}
	after := map[string]int64{
		"file1.txt": 150,
		"file2.txt": 180,
	}

	signal := GenerateDiff("snap-001", "snap-002", "tank/data", before, after)

	if signal.ModifiedFiles != 2 {
		t.Errorf("expected 2 modified files, got %d", signal.ModifiedFiles)
	}

	// ModifiedBytes = (150-100) + (180-200) = 50 + (-20) = 30
	if signal.ModifiedBytes != 30 {
		t.Errorf("expected 30 modified bytes, got %d", signal.ModifiedBytes)
	}

	if signal.AddedFiles != 0 || signal.DeletedFiles != 0 {
		t.Errorf("expected 0 added and 0 deleted, got added=%d deleted=%d",
			signal.AddedFiles, signal.DeletedFiles)
	}

	// Verify diff tree contains ~ symbol for modified files
	if !strings.Contains(signal.DiffTree, "~") {
		t.Errorf("expected diff tree to contain '~' for modified files, got: %s", signal.DiffTree)
	}
}

func TestGenerateDiff_MixedChanges(t *testing.T) {
	before := map[string]int64{
		"keep.txt":     100,
		"modified.txt": 200,
		"deleted.txt":  300,
	}
	after := map[string]int64{
		"keep.txt":     100, // unchanged
		"modified.txt": 250, // modified
		"added.txt":    400,  // added
	}

	signal := GenerateDiff("snap-001", "snap-002", "tank/data", before, after)

	if signal.AddedFiles != 1 {
		t.Errorf("expected 1 added file, got %d", signal.AddedFiles)
	}

	if signal.ModifiedFiles != 1 {
		t.Errorf("expected 1 modified file, got %d", signal.ModifiedFiles)
	}

	if signal.DeletedFiles != 1 {
		t.Errorf("expected 1 deleted file, got %d", signal.DeletedFiles)
	}

	if signal.AddedBytes != 400 {
		t.Errorf("expected 400 added bytes, got %d", signal.AddedBytes)
	}

	if signal.ModifiedBytes != 50 {
		t.Errorf("expected 50 modified bytes (250-200), got %d", signal.ModifiedBytes)
	}

	if signal.DeletedBytes != 300 {
		t.Errorf("expected 300 deleted bytes, got %d", signal.DeletedBytes)
	}

	// Verify diff tree contains all three symbols
	tree := signal.DiffTree
	if !strings.Contains(tree, "+") {
		t.Errorf("expected diff tree to contain '+' for added files")
	}
	if !strings.Contains(tree, "~") {
		t.Errorf("expected diff tree to contain '~' for modified files")
	}
	if !strings.Contains(tree, "−") {
		t.Errorf("expected diff tree to contain '−' for deleted files")
	}
}

func TestVisualizeDiff_EmptyEntries(t *testing.T) {
	result := VisualizeDiff(nil)
	if !strings.Contains(result, "(no changes)") {
		t.Errorf("expected '(no changes)' for empty entries, got: %s", result)
	}
}

func TestVisualizeDiff_WithEntries(t *testing.T) {
	entries := []DiffEntry{
		{Path: "newfile.txt", ChangeType: "added"},
		{Path: "oldfile.txt", ChangeType: "deleted"},
		{Path: "changed.txt", ChangeType: "modified", IsDir: false},
		{Path: "docs", ChangeType: "added", IsDir: true, Children: []DiffEntry{
			{Path: "docs/readme.md", ChangeType: "added"},
		}},
	}

	result := VisualizeDiff(entries)

	if !strings.Contains(result, "+ newfile.txt") {
		t.Errorf("expected '+ newfile.txt' in output")
	}
	if !strings.Contains(result, "− oldfile.txt") {
		t.Errorf("expected '− oldfile.txt' in output")
	}
	if !strings.Contains(result, "~ changed.txt") {
		t.Errorf("expected '~ changed.txt' in output")
	}
	if !strings.Contains(result, "docs/") {
		t.Errorf("expected 'docs/' for directory entry")
	}
	// Check indentation for nested entry
	if !strings.Contains(result, "  + docs/readme.md") {
		t.Errorf("expected indented nested entry '  + docs/readme.md'")
	}
}

func TestSummarizeChanges_NoChanges(t *testing.T) {
	signal := SnapDiffSignal{
		Dataset:       "tank/data",
		SnapshotBefore: "snap-001",
		SnapshotAfter:  "snap-002",
		DiffTree:       "(no changes)\n",
		CreatedAt:      time.Now(),
	}

	summary := SummarizeChanges(signal)

	if summary.TotalChanges != 0 {
		t.Errorf("expected 0 total changes, got %d", summary.TotalChanges)
	}

	if summary.ChangeTrend != "stable" {
		t.Errorf("expected stable trend, got %s", summary.ChangeTrend)
	}

	if !strings.Contains(summary.Details, "0 files changed") {
		t.Errorf("expected details to mention 0 files changed, got: %s", summary.Details)
	}
}

func TestSummarizeChanges_GrowthTrend(t *testing.T) {
	signal := SnapDiffSignal{
		Dataset:       "tank/data",
		AddedFiles:    10,
		AddedBytes:    5000,
		DiffTree:      "+ file1.txt\n+ file2.txt\n",
		CreatedAt:     time.Now(),
	}

	summary := SummarizeChanges(signal)

	if summary.ChangeTrend != "growth" {
		t.Errorf("expected growth trend, got %s", summary.ChangeTrend)
	}

	if summary.TotalChanges != 10 {
		t.Errorf("expected 10 total changes, got %d", summary.TotalChanges)
	}

	if summary.TotalBytesChanged != 5000 {
		t.Errorf("expected 5000 total bytes changed, got %d", summary.TotalBytesChanged)
	}
}

func TestSummarizeChanges_ShrinkageTrend(t *testing.T) {
	signal := SnapDiffSignal{
		Dataset:       "tank/data",
		DeletedFiles:  5,
		DeletedBytes:  3000,
		DiffTree:      "− file1.txt\n− file2.txt\n",
		CreatedAt:     time.Now(),
	}

	summary := SummarizeChanges(signal)

	if summary.ChangeTrend != "shrinkage" {
		t.Errorf("expected shrinkage trend, got %s", summary.ChangeTrend)
	}

	if summary.TotalChanges != 5 {
		t.Errorf("expected 5 total changes, got %d", summary.TotalChanges)
	}
}

func TestSummarizeChanges_MixedTrend(t *testing.T) {
	signal := SnapDiffSignal{
		Dataset:       "tank/data",
		AddedFiles:    3,
		DeletedFiles:  2,
		ModifiedFiles: 1,
		AddedBytes:    1000,
		DeletedBytes:  500,
		ModifiedBytes: 200,
		DiffTree:      "+ a.txt\n− b.txt\n~ c.txt\n",
		CreatedAt:     time.Now(),
	}

	summary := SummarizeChanges(signal)

	if summary.ChangeTrend != "mixed" {
		t.Errorf("expected mixed trend, got %s", summary.ChangeTrend)
	}

	if summary.TotalChanges != 6 {
		t.Errorf("expected 6 total changes, got %d", summary.TotalChanges)
	}

	expectedBytes := int64(1000 + 200 + 500)
	if summary.TotalBytesChanged != expectedBytes {
		t.Errorf("expected %d total bytes changed, got %d", expectedBytes, summary.TotalBytesChanged)
	}
}

func TestRecommendRetention_NoHistory(t *testing.T) {
	recs := RecommendRetention(nil)

	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}

	if recs[0].ID != "default-retention" {
		t.Errorf("expected default-retention ID, got %s", recs[0].ID)
	}

	if recs[0].KeepDays != 7 {
		t.Errorf("expected 7 keep days, got %d", recs[0].KeepDays)
	}
}

func TestRecommendRetention_StableDataset(t *testing.T) {
	history := []SnapDiffSignal{
		{Dataset: "tank/data", SnapshotBefore: "s1", SnapshotAfter: "s2"},
		{Dataset: "tank/data", SnapshotBefore: "s2", SnapshotAfter: "s3"},
		{Dataset: "tank/data", SnapshotBefore: "s3", SnapshotAfter: "s4"},
	}

	recs := RecommendRetention(history)

	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}

	if recs[0].ID != "reduce-retention-stable" {
		t.Errorf("expected reduce-retention-stable ID, got %s", recs[0].ID)
	}

	if recs[0].KeepDays != 3 {
		t.Errorf("expected 3 keep days for stable dataset, got %d", recs[0].KeepDays)
	}
}

func TestRecommendRetention_LowChangeRate(t *testing.T) {
	history := []SnapDiffSignal{
		{Dataset: "tank/data", AddedFiles: 2, SnapshotBefore: "s1", SnapshotAfter: "s2"},
		{Dataset: "tank/data", AddedFiles: 1, SnapshotBefore: "s2", SnapshotAfter: "s3"},
	}

	recs := RecommendRetention(history)

	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}

	if recs[0].ID != "standard-retention-low-change" {
		t.Errorf("expected standard-retention-low-change ID, got %s", recs[0].ID)
	}

	if recs[0].KeepDays != 7 {
		t.Errorf("expected 7 keep days for low change rate, got %d", recs[0].KeepDays)
	}
}

func TestRecommendRetention_ModerateChangeRate(t *testing.T) {
	history := []SnapDiffSignal{
		{Dataset: "tank/data", AddedFiles: 10, ModifiedFiles: 5, SnapshotBefore: "s1", SnapshotAfter: "s2"},
		{Dataset: "tank/data", AddedFiles: 15, ModifiedFiles: 8, SnapshotBefore: "s2", SnapshotAfter: "s3"},
	}

	recs := RecommendRetention(history)

	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}

	if recs[0].ID != "enhanced-retention-moderate-change" {
		t.Errorf("expected enhanced-retention-moderate-change ID, got %s", recs[0].ID)
	}

	if recs[0].KeepDays != 14 {
		t.Errorf("expected 14 keep days for moderate change rate, got %d", recs[0].KeepDays)
	}
}

func TestRecommendRetention_HighChangeRate(t *testing.T) {
	history := []SnapDiffSignal{
		{Dataset: "tank/data", AddedFiles: 30, ModifiedFiles: 20, DeletedFiles: 10, SnapshotBefore: "s1", SnapshotAfter: "s2"},
		{Dataset: "tank/data", AddedFiles: 40, ModifiedFiles: 25, DeletedFiles: 15, SnapshotBefore: "s2", SnapshotAfter: "s3"},
		{Dataset: "tank/data", AddedFiles: 50, ModifiedFiles: 30, DeletedFiles: 20, SnapshotBefore: "s3", SnapshotAfter: "s4"},
	}

	recs := RecommendRetention(history)

	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}

	foundHighChange := false
	for _, r := range recs {
		if r.ID == "aggressive-retention-high-change" {
			foundHighChange = true
			if r.KeepDays != 30 {
				t.Errorf("expected 30 keep days for high change rate, got %d", r.KeepDays)
			}
		}
	}
	if !foundHighChange {
		t.Error("expected aggressive-retention-high-change recommendation")
	}

	// Should also have frequency increase recommendation since >half snapshots significant
	foundFreq := false
	for _, r := range recs {
		if r.ID == "increase-snapshot-frequency" {
			foundFreq = true
		}
	}
	if !foundFreq {
		t.Error("expected increase-snapshot-frequency recommendation for high change rate")
	}
}

func TestDiffSymbol(t *testing.T) {
	if diffSymbol("added") != "+" {
		t.Errorf("expected '+' for added, got %s", diffSymbol("added"))
	}
	if diffSymbol("deleted") != "−" {
		t.Errorf("expected '−' for deleted, got %s", diffSymbol("deleted"))
	}
	if diffSymbol("modified") != "~" {
		t.Errorf("expected '~' for modified, got %s", diffSymbol("modified"))
	}
	if diffSymbol("unknown") != "?" {
		t.Errorf("expected '?' for unknown, got %s", diffSymbol("unknown"))
	}
}