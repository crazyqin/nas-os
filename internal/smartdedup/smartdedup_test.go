package smartdedup

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- types_test ---

func TestContentType_String(t *testing.T) {
	tests := []struct {
		ct   ContentType
		want string
	}{
		{ContentTypeImage, "image"},
		{ContentTypeAudio, "audio"},
		{ContentTypeVideo, "video"},
		{ContentTypeDocument, "document"},
		{ContentTypeArchive, "archive"},
		{ContentTypeBinary, "binary"},
		{ContentTypeUnknown, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("ContentType(%d).String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestRetentionPolicy_String(t *testing.T) {
	tests := []struct {
		rp   RetentionPolicy
		want string
	}{
		{RetainNewest, "newest"},
		{RetainOldest, "oldest"},
		{RetainLargest, "largest"},
		{RetainSmallest, "smallest"},
		{RetainMostUsed, "most_used"},
		{RetainShortestPath, "shortest_path"},
	}
	for _, tt := range tests {
		if got := tt.rp.String(); got != tt.want {
			t.Errorf("RetentionPolicy(%d).String() = %q, want %q", tt.rp, got, tt.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("DefaultConfig.Enabled should be true")
	}
	if cfg.RetentionPolicy != RetainNewest {
		t.Errorf("DefaultConfig.RetentionPolicy = %v, want RetainNewest", cfg.RetentionPolicy)
	}
	if !cfg.SafeDelete {
		t.Error("DefaultConfig.SafeDelete should be true")
	}
	if cfg.MaxWorkers <= 0 {
		t.Error("DefaultConfig.MaxWorkers should be > 0")
	}
}

func TestDedupStats_Snapshot(t *testing.T) {
	stats := &DedupStats{
		TotalScans:        5,
		TotalFilesScanned: 100,
		TotalSizeScanned:  1024 * 1024,
		TotalSavedBytes:   512 * 1024,
	}
	snap := stats.Snapshot()
	if snap.TotalScans != 5 {
		t.Errorf("Snapshot.TotalScans = %d, want 5", snap.TotalScans)
	}
	// 修改原值不应影响快照
	stats.TotalScans = 10
	if snap.TotalScans != 5 {
		t.Error("Snapshot should be independent of original")
	}
}

func TestDedupStats_AddScan(t *testing.T) {
	stats := &DedupStats{}
	result := &ScanResult{
		TotalFiles:     50,
		TotalSize:      1024 * 1024,
		DuplicateCount: 10,
		DuplicateSize:  256 * 1024,
		EndTime:        time.Now(),
		Duration:       time.Second * 5,
	}
	stats.AddScan(result)

	if stats.TotalScans != 1 {
		t.Errorf("TotalScans = %d, want 1", stats.TotalScans)
	}
	if stats.TotalFilesScanned != 50 {
		t.Errorf("TotalFilesScanned = %d, want 50", stats.TotalFilesScanned)
	}
	if stats.TotalDuplicates != 10 {
		t.Errorf("TotalDuplicates = %d, want 10", stats.TotalDuplicates)
	}
}

// --- hasher_test ---

func TestClassifyContentType(t *testing.T) {
	tests := []struct {
		ext  string
		want ContentType
	}{
		{".jpg", ContentTypeImage},
		{".PNG", ContentTypeImage},
		{".mp3", ContentTypeAudio},
		{".FLAC", ContentTypeAudio},
		{".mp4", ContentTypeVideo},
		{".pdf", ContentTypeDocument},
		{".zip", ContentTypeArchive},
		{".exe", ContentTypeBinary},
		{"", ContentTypeBinary},
	}
	for _, tt := range tests {
		if got := ClassifyContentType(tt.ext); got != tt.want {
			t.Errorf("ClassifyContentType(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestHasher_ContentHash(t *testing.T) {
	dir := t.TempDir()
	content := []byte("hello world smartdedup test content")
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	hasher := NewHasher(0)
	hash1, err := hasher.ContentHash(filePath)
	if err != nil {
		t.Fatal(err)
	}

	// 手动计算期望哈希
	expected := sha256.Sum256(content)
	expectedHex := hex.EncodeToString(expected[:])

	if hash1 != expectedHex {
		t.Errorf("ContentHash = %s, want %s", hash1, expectedHex)
	}

	// 同一文件两次哈希应一致
	hash2, err := hasher.ContentHash(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Error("ContentHash should be deterministic")
	}
}

func TestHasher_ContentHash_SameContent(t *testing.T) {
	dir := t.TempDir()
	content := []byte("duplicate content for testing")

	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")
	os.WriteFile(file1, content, 0644)
	os.WriteFile(file2, content, 0644)

	hasher := NewHasher(0)
	h1, _ := hasher.ContentHash(file1)
	h2, _ := hasher.ContentHash(file2)

	if h1 != h2 {
		t.Error("Same content should produce same hash")
	}
}

func TestHasher_ContentHash_DifferentContent(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")
	os.WriteFile(file1, []byte("content A"), 0644)
	os.WriteFile(file2, []byte("content B"), 0644)

	hasher := NewHasher(0)
	h1, _ := hasher.ContentHash(file1)
	h2, _ := hasher.ContentHash(file2)

	if h1 == h2 {
		t.Error("Different content should produce different hash")
	}
}

func TestHasher_ComputeFileInfo(t *testing.T) {
	dir := t.TempDir()
	content := []byte("test file info")
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, content, 0644)

	hasher := NewHasher(0)
	fi, err := hasher.ComputeFileInfo(filePath)
	if err != nil {
		t.Fatal(err)
	}

	if fi.Path != filePath {
		t.Errorf("Path = %s, want %s", fi.Path, filePath)
	}
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", fi.Size, len(content))
	}
	if fi.ContentHash == "" {
		t.Error("ContentHash should not be empty")
	}
	if fi.ContentType != ContentTypeDocument {
		t.Errorf("ContentType = %v, want Document", fi.ContentType)
	}
}

func TestHasher_ComputeFileInfo_Dir(t *testing.T) {
	dir := t.TempDir()
	hasher := NewHasher(0)
	_, err := hasher.ComputeFileInfo(dir)
	if err == nil {
		t.Error("ComputeFileInfo on directory should return error")
	}
}

func TestHasher_PartialContentHash(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("partial hash test content"), 0644)

	hasher := NewHasher(0)
	h, err := hasher.PartialContentHash(filePath, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Error("PartialContentHash should not be empty")
	}
}

func TestHasher_ContentHashReader(t *testing.T) {
	hasher := NewHasher(0)
	content := []byte("reader test content")
	h1, _ := hasher.ContentHashReader(stringReader(content))

	expected := sha256.Sum256(content)
	expectedHex := hex.EncodeToString(expected[:])

	if h1 != expectedHex {
		t.Errorf("ContentHashReader = %s, want %s", h1, expectedHex)
	}
}

// --- scanner_test ---

func TestScanner_Scan_EmptyPaths(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ScanPaths = []string{}
	scanner := NewScanner(cfg)
	_, err := scanner.Scan()
	if err == nil {
		t.Error("Scan with empty paths should return error")
	}
}

func TestScanner_Scan_SingleDir(t *testing.T) {
	dir := t.TempDir()
	content := []byte("scanner test content")

	// 创建两个相同内容的文件
	os.WriteFile(filepath.Join(dir, "a.txt"), content, 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), content, 0644)
	// 创建一个不同内容的文件
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("different content"), 0644)

	cfg := DefaultConfig()
	cfg.ScanPaths = []string{dir}
	cfg.ExcludePaths = nil // 清除排除路径以允许测试临时目录
	cfg.PerceptualEnabled = false
	cfg.MaxWorkers = 2

	scanner := NewScanner(cfg)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", result.TotalFiles)
	}
	if result.DuplicateCount != 1 {
		t.Errorf("DuplicateCount = %d, want 1", result.DuplicateCount)
	}
	if len(result.DuplicateGroups) != 1 {
		t.Errorf("DuplicateGroups = %d, want 1", len(result.DuplicateGroups))
	}
	if len(result.DuplicateGroups[0].Files) != 2 {
		t.Errorf("Duplicate group size = %d, want 2", len(result.DuplicateGroups[0].Files))
	}
}

func TestScanner_Scan_ExcludePatterns(t *testing.T) {
	dir := t.TempDir()
	content := []byte("exclude test")

	os.WriteFile(filepath.Join(dir, "good.txt"), content, 0644)
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "config"), content, 0644)

	cfg := DefaultConfig()
	cfg.ScanPaths = []string{dir}
	cfg.ExcludePaths = nil
	cfg.ExcludePatterns = []string{".git"}
	cfg.MaxWorkers = 1

	scanner := NewScanner(cfg)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1 (should exclude .git)", result.TotalFiles)
	}
}

func TestScanner_Scan_FileSizeFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "small.txt"), []byte("hi"), 0644)
	os.WriteFile(filepath.Join(dir, "large.txt"), []byte("this is a larger file content for testing"), 0644)

	cfg := DefaultConfig()
	cfg.ScanPaths = []string{dir}
	cfg.ExcludePaths = nil
	cfg.MinFileSize = 10
	cfg.MaxWorkers = 1

	scanner := NewScanner(cfg)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1 (small file filtered)", result.TotalFiles)
	}
}

func TestScanner_ScanSingle(t *testing.T) {
	dir := t.TempDir()
	content := []byte("single scan test")
	filePath := filepath.Join(dir, "single.txt")
	os.WriteFile(filePath, content, 0644)

	scanner := NewScanner(DefaultConfig())
	fi, err := scanner.ScanSingle(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Path != filePath {
		t.Errorf("Path = %s, want %s", fi.Path, filePath)
	}
}

// --- strategy_test ---

func TestStrategy_Select_Newest(t *testing.T) {
	now := time.Now()
	group := &DuplicateGroup{
		ContentHash: "abc123",
		Files: []*FileInfo{
			{Path: "/old.txt", ModTime: now.Add(-time.Hour * 24), Size: 100},
			{Path: "/new.txt", ModTime: now, Size: 100},
			{Path: "/mid.txt", ModTime: now.Add(-time.Hour * 12), Size: 100},
		},
	}

	strategy := NewStrategy(RetainNewest)
	sel := strategy.Select(group)
	if sel == nil {
		t.Fatal("Select returned nil")
	}
	if sel.Keep.Path != "/new.txt" {
		t.Errorf("Keep = %s, want /new.txt", sel.Keep.Path)
	}
	if len(sel.Remove) != 2 {
		t.Errorf("Remove count = %d, want 2", len(sel.Remove))
	}
}

func TestStrategy_Select_Oldest(t *testing.T) {
	now := time.Now()
	group := &DuplicateGroup{
		ContentHash: "abc123",
		Files: []*FileInfo{
			{Path: "/old.txt", ModTime: now.Add(-time.Hour * 24), Size: 100},
			{Path: "/new.txt", ModTime: now, Size: 100},
		},
	}

	strategy := NewStrategy(RetainOldest)
	sel := strategy.Select(group)
	if sel.Keep.Path != "/old.txt" {
		t.Errorf("Keep = %s, want /old.txt", sel.Keep.Path)
	}
}

func TestStrategy_Select_Largest(t *testing.T) {
	group := &DuplicateGroup{
		ContentHash: "abc123",
		Files: []*FileInfo{
			{Path: "/small.txt", Size: 100},
			{Path: "/large.txt", Size: 1000},
		},
	}

	strategy := NewStrategy(RetainLargest)
	sel := strategy.Select(group)
	if sel.Keep.Path != "/large.txt" {
		t.Errorf("Keep = %s, want /large.txt", sel.Keep.Path)
	}
}

func TestStrategy_Select_Smallest(t *testing.T) {
	group := &DuplicateGroup{
		ContentHash: "abc123",
		Files: []*FileInfo{
			{Path: "/small.txt", Size: 100},
			{Path: "/large.txt", Size: 1000},
		},
	}

	strategy := NewStrategy(RetainSmallest)
	sel := strategy.Select(group)
	if sel.Keep.Path != "/small.txt" {
		t.Errorf("Keep = %s, want /small.txt", sel.Keep.Path)
	}
}

func TestStrategy_Select_MostUsed(t *testing.T) {
	group := &DuplicateGroup{
		ContentHash: "abc123",
		Files: []*FileInfo{
			{Path: "/rare.txt", UsageCount: 1},
			{Path: "/common.txt", UsageCount: 100},
		},
	}

	strategy := NewStrategy(RetainMostUsed)
	sel := strategy.Select(group)
	if sel.Keep.Path != "/common.txt" {
		t.Errorf("Keep = %s, want /common.txt", sel.Keep.Path)
	}
}

func TestStrategy_Select_ShortestPath(t *testing.T) {
	group := &DuplicateGroup{
		ContentHash: "abc123",
		Files: []*FileInfo{
			{Path: "/very/long/path/to/file.txt"},
			{Path: "/short.txt"},
		},
	}

	strategy := NewStrategy(RetainShortestPath)
	sel := strategy.Select(group)
	if sel.Keep.Path != "/short.txt" {
		t.Errorf("Keep = %s, want /short.txt", sel.Keep.Path)
	}
}

func TestStrategy_Select_Nil(t *testing.T) {
	strategy := NewStrategy(RetainNewest)
	if sel := strategy.Select(nil); sel != nil {
		t.Error("Select(nil) should return nil")
	}
}

func TestStrategy_Select_SingleFile(t *testing.T) {
	group := &DuplicateGroup{
		ContentHash: "abc123",
		Files:       []*FileInfo{{Path: "/only.txt"}},
	}
	strategy := NewStrategy(RetainNewest)
	if sel := strategy.Select(group); sel != nil {
		t.Error("Select with single file should return nil")
	}
}

func TestStrategy_SelectAll(t *testing.T) {
	groups := []*DuplicateGroup{
		{
			ContentHash: "a",
			Files: []*FileInfo{
				{Path: "/a1.txt", Size: 100, ModTime: time.Now()},
				{Path: "/a2.txt", Size: 100, ModTime: time.Now().Add(-time.Hour)},
			},
		},
		{
			ContentHash: "b",
			Files: []*FileInfo{
				{Path: "/b1.txt", Size: 200, ModTime: time.Now()},
				{Path: "/b2.txt", Size: 200, ModTime: time.Now().Add(-time.Hour)},
			},
		},
	}

	strategy := NewStrategy(RetainNewest)
	selections := strategy.SelectAll(groups)
	if len(selections) != 2 {
		t.Errorf("SelectAll returned %d selections, want 2", len(selections))
	}
}

func TestStrategy_EstimateSaving(t *testing.T) {
	groups := []*DuplicateGroup{
		{
			Files: []*FileInfo{
				{Size: 1000},
				{Size: 1000},
				{Size: 1000},
			},
		},
		{
			Files: []*FileInfo{
				{Size: 500},
				{Size: 500},
			},
		},
	}

	strategy := NewStrategy(RetainNewest)
	saving := strategy.EstimateSaving(groups)
	// 第一组: 2 * 1000 = 2000, 第二组: 1 * 500 = 500
	if saving != 2500 {
		t.Errorf("EstimateSaving = %d, want 2500", saving)
	}
}

func TestSortGroupsBySaving(t *testing.T) {
	groups := []*DuplicateGroup{
		{SavedSize: 100},
		{SavedSize: 500},
		{SavedSize: 200},
	}
	SortGroupsBySaving(groups)
	if groups[0].SavedSize != 500 {
		t.Errorf("First group SavedSize = %d, want 500", groups[0].SavedSize)
	}
	if groups[2].SavedSize != 100 {
		t.Errorf("Last group SavedSize = %d, want 100", groups[2].SavedSize)
	}
}

func TestFilterGroupsByMinSize(t *testing.T) {
	groups := []*DuplicateGroup{
		{SavedSize: 100},
		{SavedSize: 500},
		{SavedSize: 200},
	}
	filtered := FilterGroupsByMinSize(groups, 200)
	if len(filtered) != 2 {
		t.Errorf("Filtered count = %d, want 2", len(filtered))
	}
}

func TestFilterGroupsByContentType(t *testing.T) {
	groups := []*DuplicateGroup{
		{Files: []*FileInfo{{ContentType: ContentTypeImage}}},
		{Files: []*FileInfo{{ContentType: ContentTypeDocument}}},
		{Files: []*FileInfo{{ContentType: ContentTypeImage}}},
	}
	filtered := FilterGroupsByContentType(groups, ContentTypeImage)
	if len(filtered) != 2 {
		t.Errorf("Filtered count = %d, want 2", len(filtered))
	}
}

// --- engine_test ---

func TestEngine_Scan(t *testing.T) {
	dir := t.TempDir()
	content := []byte("engine test content")
	os.WriteFile(filepath.Join(dir, "a.txt"), content, 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), content, 0644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("unique"), 0644)

	cfg := DefaultConfig()
	cfg.ScanPaths = []string{dir}
	cfg.ExcludePaths = nil
	cfg.MaxWorkers = 1

	engine := NewEngine(cfg)
	result, err := engine.Scan()
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalFiles != 3 {
		t.Errorf("TotalFiles = %d, want 3", result.TotalFiles)
	}
	if result.DuplicateCount != 1 {
		t.Errorf("DuplicateCount = %d, want 1", result.DuplicateCount)
	}
}

func TestEngine_Scan_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	engine := NewEngine(cfg)
	_, err := engine.Scan()
	if err == nil {
		t.Error("Scan on disabled engine should return error")
	}
}

func TestEngine_Scan_EmptyPaths(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ScanPaths = []string{}
	engine := NewEngine(cfg)
	_, err := engine.Scan()
	if err == nil {
		t.Error("Scan with empty paths should return error")
	}
}

func TestEngine_Stats(t *testing.T) {
	dir := t.TempDir()
	content := []byte("stats test")
	os.WriteFile(filepath.Join(dir, "a.txt"), content, 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), content, 0644)

	cfg := DefaultConfig()
	cfg.ScanPaths = []string{dir}
	cfg.ExcludePaths = nil
	cfg.MaxWorkers = 1

	engine := NewEngine(cfg)
	engine.Scan()

	stats := engine.Stats()
	if stats.TotalScans != 1 {
		t.Errorf("TotalScans = %d, want 1", stats.TotalScans)
	}
	if stats.TotalFilesScanned != 2 {
		t.Errorf("TotalFilesScanned = %d, want 2", stats.TotalFilesScanned)
	}
}

func TestEngine_Config_Update(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngine(cfg)

	newCfg := DefaultConfig()
	newCfg.MaxWorkers = 8
	newCfg.SafeDelete = false
	engine.UpdateConfig(newCfg)

	got := engine.Config()
	if got.MaxWorkers != 8 {
		t.Errorf("MaxWorkers = %d, want 8", got.MaxWorkers)
	}
	if got.SafeDelete {
		t.Error("SafeDelete should be false after update")
	}
}

func TestEngine_Dedup_DryRun(t *testing.T) {
	dir := t.TempDir()
	content := []byte("dedup dry run test")
	os.WriteFile(filepath.Join(dir, "a.txt"), content, 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), content, 0644)

	cfg := DefaultConfig()
	cfg.ScanPaths = []string{dir}
	cfg.ExcludePaths = nil
	cfg.DryRun = true
	cfg.SafeDelete = false
	cfg.MaxWorkers = 1

	engine := NewEngine(cfg)
	result, err := engine.Dedup()
	if err != nil {
		t.Fatal(err)
	}

	if result.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1 (dry run)", result.Deleted)
	}
	if result.SavedBytes != int64(len(content)) {
		t.Errorf("SavedBytes = %d, want %d", result.SavedBytes, len(content))
	}

	// 文件应仍然存在（dry run）
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
		t.Error("a.txt should still exist in dry run")
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Error("b.txt should still exist in dry run")
	}
}

func TestEngine_Dedup_SafeDelete(t *testing.T) {
	dir := t.TempDir()
	trashDir := filepath.Join(dir, "trash")
	content := []byte("safe delete test")
	os.WriteFile(filepath.Join(dir, "a.txt"), content, 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), content, 0644)

	cfg := DefaultConfig()
	cfg.ScanPaths = []string{dir}
	cfg.ExcludePaths = nil
	cfg.DryRun = false
	cfg.SafeDelete = true
	cfg.TrashPath = trashDir
	cfg.MaxWorkers = 1

	engine := NewEngine(cfg)
	result, err := engine.Dedup()
	if err != nil {
		t.Fatal(err)
	}

	if result.Trashed != 1 {
		t.Errorf("Trashed = %d, want 1", result.Trashed)
	}

	// 检查回收站
	entries, _ := os.ReadDir(trashDir)
	if len(entries) != 1 {
		t.Errorf("Trash entries = %d, want 1", len(entries))
	}
}

func TestEngine_Dedup_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	engine := NewEngine(cfg)
	_, err := engine.Dedup()
	if err == nil {
		t.Error("Dedup on disabled engine should return error")
	}
}

func TestEngine_IsScanning(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	if engine.IsScanning() {
		t.Error("New engine should not be scanning")
	}
}

func TestEngine_ScanSingle(t *testing.T) {
	dir := t.TempDir()
	content := []byte("scan single test")
	filePath := filepath.Join(dir, "single.txt")
	os.WriteFile(filePath, content, 0644)

	engine := NewEngine(DefaultConfig())
	fi, err := engine.ScanSingle(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Path != filePath {
		t.Errorf("Path = %s, want %s", fi.Path, filePath)
	}
}

func TestEngine_CleanTrash(t *testing.T) {
	trashDir := t.TempDir()

	// 创建一个旧文件
	oldFile := filepath.Join(trashDir, "old.txt")
	os.WriteFile(oldFile, []byte("old"), 0644)
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldFile, oldTime, oldTime)

	// 创建一个新文件
	newFile := filepath.Join(trashDir, "new.txt")
	os.WriteFile(newFile, []byte("new"), 0644)

	cfg := DefaultConfig()
	cfg.TrashPath = trashDir
	engine := NewEngine(cfg)

	removed, err := engine.CleanTrash(5) // 清理超过 5 天的
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("Removed = %d, want 1", removed)
	}

	// 新文件应仍在
	if _, err := os.Stat(newFile); err != nil {
		t.Error("New file should still exist")
	}
}

// --- helper ---

// simpleReader 从 []byte 创建一个简单的 Reader。
type simpleReader struct {
	data []byte
	pos  int
}

func (r *simpleReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func stringReader(data []byte) *simpleReader {
	return &simpleReader{data: data}
}
