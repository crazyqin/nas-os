package aidatadedup

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func TestNewManager(t *testing.T) {
	m := setupTestManager(t)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.IsRunning() {
		t.Error("expected manager not running initially")
	}
}

func TestManagerStartStop(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !m.IsRunning() {
		t.Error("expected manager running after start")
	}

	// 重复启动应报错
	if err := m.Start(ctx); err == nil {
		t.Error("expected error for double start")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if m.IsRunning() {
		t.Error("expected manager stopped after stop")
	}

	// 重复停止应报错
	if err := m.Stop(); err == nil {
		t.Error("expected error for double stop")
	}
}

func TestManagerDisabled(t *testing.T) {
	cfg := DefaultDedupConfig()
	cfg.Enabled = false
	m := NewManager(zap.NewNop(), cfg)

	if err := m.Start(context.Background()); err == nil {
		t.Error("expected error when disabled")
	}
}

func TestDefaultDedupConfig(t *testing.T) {
	cfg := DefaultDedupConfig()

	if !cfg.Enabled {
		t.Error("expected enabled to be true")
	}
	if cfg.DefaultStrategy != StrategyAuto {
		t.Errorf("expected strategy %s, got %s", StrategyAuto, cfg.DefaultStrategy)
	}
	if cfg.SimilarityThreshold != 0.85 {
		t.Errorf("expected threshold 0.85, got %f", cfg.SimilarityThreshold)
	}
	if cfg.MaxConcurrentScans != 3 {
		t.Errorf("expected max concurrent 3, got %d", cfg.MaxConcurrentScans)
	}
	if !cfg.EnableAI {
		t.Error("expected AI enabled")
	}
	if cfg.AIConfidenceThreshold != 0.9 {
		t.Errorf("expected AI threshold 0.9, got %f", cfg.AIConfidenceThreshold)
	}
}

func TestScan(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	req := &DedupRequest{
		Paths:              []string{"/data/photos"},
		Strategy:           StrategyAuto,
		SimilarityThreshold: 0.9,
		Recursive:          true,
	}

	result, err := m.Scan(ctx, req)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if result.ID == "" {
		t.Error("expected non-empty scan ID")
	}
	if result.Status != ScanStatusRunning && result.Status != ScanStatusComplete {
		t.Errorf("unexpected status: %v", result.Status)
	}

	// 等待扫描完成
	time.Sleep(200 * time.Millisecond)

	got, err := m.GetScanResult(result.ID)
	if err != nil {
		t.Fatalf("GetScanResult failed: %v", err)
	}
	if got.Status != ScanStatusComplete {
		t.Errorf("expected status complete, got %v", got.Status)
	}
}

func TestScanNotRunning(t *testing.T) {
	m := setupTestManager(t)

	req := &DedupRequest{
		Paths: []string{"/data"},
	}

	_, err := m.Scan(context.Background(), req)
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestListScanResults(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 执行多次扫描
	for i := 0; i < 3; i++ {
		m.Scan(ctx, &DedupRequest{
			Paths: []string{"/data/test"},
		})
	}

	results := m.ListScanResults()
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestDuplicateGroupOperations(t *testing.T) {
	m := setupTestManager(t)

	// 添加重复组
	group := &DuplicateGroup{
		ID:    "group-001",
		Files: []*FileEntry{
			{ID: "file-001", Path: "/data/photo1.jpg", Size: 1024},
			{ID: "file-002", Path: "/data/photo1_copy.jpg", Size: 1024},
		},
		Similarity:  0.98,
		DedupType:   StrategyFuzzyMatch,
		Status:      StatusPending,
		TotalSize:   2048,
		SaveableSize: 1024,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.AddDuplicateGroup(group)

	// 获取组
	got, err := m.GetDuplicateGroup("group-001")
	if err != nil {
		t.Fatalf("GetDuplicateGroup failed: %v", err)
	}
	if got.Similarity != 0.98 {
		t.Errorf("expected similarity 0.98, got %f", got.Similarity)
	}

	// 列出所有组
	groups := m.ListDuplicateGroups()
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}

	// 解决重复组
	if err := m.ResolveDuplicateGroup("group-001", "file-001"); err != nil {
		t.Fatalf("ResolveDuplicateGroup failed: %v", err)
	}

	got, _ = m.GetDuplicateGroup("group-001")
	if got.Status != StatusResolved {
		t.Errorf("expected status resolved, got %v", got.Status)
	}
}

func TestResolveDuplicateGroupNotFound(t *testing.T) {
	m := setupTestManager(t)

	err := m.ResolveDuplicateGroup("nonexistent", "file-001")
	if err == nil {
		t.Error("expected error for nonexistent group")
	}
}

func TestResolveDuplicateGroupFileNotFound(t *testing.T) {
	m := setupTestManager(t)

	group := &DuplicateGroup{
		ID: "group-002",
		Files: []*FileEntry{
			{ID: "file-001", Path: "/data/test.jpg", Size: 100},
		},
		Status: StatusPending,
	}
	m.AddDuplicateGroup(group)

	err := m.ResolveDuplicateGroup("group-002", "nonexistent-file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestMergeFiles(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 添加测试组
	group := &DuplicateGroup{
		ID: "merge-group-001",
		Files: []*FileEntry{
			{ID: "f1", Path: "/data/old.jpg", Size: 1000, ModTime: time.Now().Add(-24 * time.Hour)},
			{ID: "f2", Path: "/data/new.jpg", Size: 1200, ModTime: time.Now()},
		},
		Similarity:  0.95,
		Status:      StatusPending,
		TotalSize:   2200,
		SaveableSize: 1000,
	}
	m.AddDuplicateGroup(group)

	report, err := m.MergeFiles(ctx, &MergeRequest{
		GroupID:      "merge-group-001",
		Strategy:     MergeKeepNewest,
		DeleteOthers: true,
	})
	if err != nil {
		t.Fatalf("MergeFiles failed: %v", err)
	}
	if report.FilesDeleted != 1 {
		t.Errorf("expected 1 file deleted, got %d", report.FilesDeleted)
	}
	if report.SpaceSaved != 1000 {
		t.Errorf("expected space saved 1000, got %d", report.SpaceSaved)
	}
}

func TestMergeFilesNotRunning(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.MergeFiles(context.Background(), &MergeRequest{
		GroupID: "test",
	})
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestMergeFilesGroupNotFound(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	_, err := m.MergeFiles(ctx, &MergeRequest{
		GroupID: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent group")
	}
}

func TestAnalyzeFile(t *testing.T) {
	m := setupTestManager(t)

	file := &FileEntry{
		ID:       "analyze-001",
		Path:     "/data/test.jpg",
		FileType: FileTypeImage,
		Size:     1024,
	}

	result, err := m.AnalyzeFile(context.Background(), file)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	if result.FileID != file.ID {
		t.Errorf("expected file ID %s, got %s", file.ID, result.FileID)
	}
	if result.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", result.Confidence)
	}
}

func TestAnalyzeFileAIDisabled(t *testing.T) {
	cfg := DefaultDedupConfig()
	cfg.EnableAI = false
	m := NewManager(zap.NewNop(), cfg)

	file := &FileEntry{
		ID:       "test-001",
		FileType: FileTypeImage,
	}

	_, err := m.AnalyzeFile(context.Background(), file)
	if err == nil {
		t.Error("expected error when AI disabled")
	}
}

func TestReports(t *testing.T) {
	m := setupTestManager(t)
	ctx := context.Background()

	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer m.Stop()

	// 创建报告
	group := &DuplicateGroup{
		ID: "report-group-001",
		Files: []*FileEntry{
			{ID: "rf1", Size: 500, ModTime: time.Now()},
			{ID: "rf2", Size: 500, ModTime: time.Now()},
		},
		Status:      StatusPending,
		SaveableSize: 500,
	}
	m.AddDuplicateGroup(group)

	report, _ := m.MergeFiles(ctx, &MergeRequest{
		GroupID: "report-group-001",
	})

	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("expected report ID %s, got %s", report.ID, got.ID)
	}

	reports := m.ListReports()
	if len(reports) != 1 {
		t.Errorf("expected 1 report, got %d", len(reports))
	}
}

func TestGetReportNotFound(t *testing.T) {
	m := setupTestManager(t)

	_, err := m.GetReport("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent report")
	}
}

func TestConfigOperations(t *testing.T) {
	m := setupTestManager(t)

	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("expected enabled")
	}

	newCfg := DefaultDedupConfig()
	newCfg.SimilarityThreshold = 0.95
	m.UpdateConfig(newCfg)

	cfg = m.GetConfig()
	if cfg.SimilarityThreshold != 0.95 {
		t.Errorf("expected threshold 0.95, got %f", cfg.SimilarityThreshold)
	}
}

func TestGetStats(t *testing.T) {
	m := setupTestManager(t)

	stats := m.GetStats()
	if stats["running"] != false {
		t.Error("expected running false")
	}
	if stats["scan_results"] != 0 {
		t.Error("expected 0 scan results")
	}
}

func TestSelectBestFile(t *testing.T) {
	m := setupTestManager(t)

	now := time.Now()
	files := []*FileEntry{
		{ID: "old", ModTime: now.Add(-24 * time.Hour), Size: 100},
		{ID: "new", ModTime: now, Size: 200},
		{ID: "mid", ModTime: now.Add(-12 * time.Hour), Size: 150},
	}

	// Keep newest
	best := m.selectBestFile(files, MergeKeepNewest)
	if best.ID != "new" {
		t.Errorf("expected 'new', got '%s'", best.ID)
	}

	// Keep oldest
	best = m.selectBestFile(files, MergeKeepOldest)
	if best.ID != "old" {
		t.Errorf("expected 'old', got '%s'", best.ID)
	}

	// Keep largest
	best = m.selectBestFile(files, MergeKeepLargest)
	if best.ID != "new" {
		t.Errorf("expected 'new', got '%s'", best.ID)
	}

	// Empty files
	best = m.selectBestFile(nil, MergeKeepNewest)
	if best != nil {
		t.Error("expected nil for empty files")
	}
}

func TestComputeHash(t *testing.T) {
	data := []byte("hello world")
	hash1 := computeHash(data)
	hash2 := computeHash(data)

	if hash1 != hash2 {
		t.Error("expected same hash for same data")
	}
	if len(hash1) != 64 { // SHA256 hex
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if len(id1) != 32 { // 16 bytes hex
		t.Errorf("expected ID length 32, got %d", len(id1))
	}
}
