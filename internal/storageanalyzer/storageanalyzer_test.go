package storageanalyzer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// createTestDir creates a temporary test directory structure
func createTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"documents/report.pdf":  "pdf content",
		"documents/readme.md":   "markdown content",
		"images/photo.jpg":      "jpg content",
		"images/logo.png":       "png content",
		"videos/movie.mp4":      "mp4 content large",
		"music/song.mp3":        "mp3 content",
		"code/main.go":          "package main",
		"code/utils.go":         "package main",
		"archives/backup.tar.gz": "tar gz content",
		"logs/app.log":          "log content",
		"logs/error.log":        "error log content",
		"duplicate1.txt":        "duplicate content",
		"subdir/duplicate2.txt": "duplicate content",
		"empty.txt":             "",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return tmpDir
}

func TestNewAnalyzer(t *testing.T) {
	config := DefaultConfig()
	analyzer := NewAnalyzer(config)

	if analyzer == nil {
		t.Fatal("NewAnalyzer returned nil")
	}

	if analyzer.config.HashAlgorithm != "sha256" {
		t.Errorf("Expected hash algorithm sha256, got %s", analyzer.config.HashAlgorithm)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.HashAlgorithm != "sha256" {
		t.Errorf("Expected hash algorithm sha256, got %s", config.HashAlgorithm)
	}

	if config.MaxFileSize != 10*1024*1024*1024 {
		t.Errorf("Expected max file size 10GB, got %d", config.MaxFileSize)
	}

	if config.ScheduleCron != "0 2 * * *" {
		t.Errorf("Expected schedule cron '0 2 * * *', got %s", config.ScheduleCron)
	}
}

func TestFileTypeCategory(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".pdf", "documents"},
		{".jpg", "images"},
		{".mp4", "videos"},
		{".mp3", "audio"},
		{".zip", "archives"},
		{".go", "code"},
		{".db", "databases"},
		{".log", "system"},
		{".xyz", ""},
	}

	for _, tt := range tests {
		cat, ok := FileTypeCategory[tt.ext]
		if tt.expected == "" {
			if ok {
				t.Errorf("Expected no category for %s, got %s", tt.ext, cat)
			}
		} else {
			if !ok || cat != tt.expected {
				t.Errorf("Expected category %s for %s, got %s", tt.expected, tt.ext, cat)
			}
		}
	}
}

func TestRunAnalysis(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		MaxFileSize:   1024 * 1024 * 1024,
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	report, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	if report == nil {
		t.Fatal("Report is nil")
	}

	if report.ID == "" {
		t.Error("Report ID is empty")
	}

	if report.GeneratedAt.IsZero() {
		t.Error("Report GeneratedAt is zero")
	}

	if report.UsedSpace <= 0 {
		t.Error("UsedSpace should be positive")
	}

	if len(report.ByFileType) == 0 {
		t.Error("ByFileType should not be empty")
	}
}

func TestGetReport(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	report, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	// Should find the report
	found, ok := analyzer.GetReport(report.ID)
	if !ok {
		t.Error("Report not found")
	}
	if found.ID != report.ID {
		t.Errorf("Expected report ID %s, got %s", report.ID, found.ID)
	}

	// Should not find non-existent report
	_, ok = analyzer.GetReport("non-existent")
	if ok {
		t.Error("Should not find non-existent report")
	}
}

func TestGetReports(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	// Run analysis twice
	_, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	_, err = analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	reports := analyzer.GetReports()
	if len(reports) != 2 {
		t.Errorf("Expected 2 reports, got %d", len(reports))
	}
}

func TestGetHistory(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	_, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	history := analyzer.GetHistory()
	if len(history) == 0 {
		t.Error("History should not be empty after analysis")
	}
}

func TestIsExcluded(t *testing.T) {
	config := AnalysisConfig{
		ExcludePaths: []string{"/proc", "/sys", "/dev"},
	}

	analyzer := NewAnalyzer(config)

	tests := []struct {
		path     string
		expected bool
	}{
		{"/proc/cpuinfo", true},
		{"/sys/class", true},
		{"/dev/null", true},
		{"/home/user/file", false},
		{"/tmp/test", false},
	}

	for _, tt := range tests {
		result := analyzer.isExcluded(tt.path)
		if result != tt.expected {
			t.Errorf("isExcluded(%s) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestHandlers(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	handlers := NewHandlers(analyzer)

	// Setup router
	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	// Test health endpoint
	t.Run("Health", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/storage-analyzer/health", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if _, ok := resp["status"]; !ok {
			t.Error("Response should contain 'status'")
		}
	})

	// Test current usage endpoint
	t.Run("CurrentUsage", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/storage-analyzer/current", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if _, ok := resp["total_space"]; !ok {
			t.Error("Response should contain 'total_space'")
		}
	})

	// Test start analysis
	t.Run("StartAnalysis", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/storage-analyzer/analyze", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected status 202, got %d", w.Code)
		}
	})

	// Test reports endpoint
	t.Run("ListReports", func(t *testing.T) {
		// Run analysis first
		ctx := context.Background()
		_, err := analyzer.RunAnalysis(ctx)
		if err != nil {
			t.Fatalf("RunAnalysis failed: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/storage-analyzer/reports", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)

		if _, ok := resp["reports"]; !ok {
			t.Error("Response should contain 'reports'")
		}
	})
}

func TestReportEndpoints(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	report, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	handlers := NewHandlers(analyzer)
	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	endpoints := []struct {
		name string
		path string
	}{
		{"directories", "/api/v1/storage-analyzer/reports/" + report.ID + "/directories"},
		{"file-types", "/api/v1/storage-analyzer/reports/" + report.ID + "/file-types"},
		{"users", "/api/v1/storage-analyzer/reports/" + report.ID + "/users"},
		{"time-usage", "/api/v1/storage-analyzer/reports/" + report.ID + "/time-usage"},
		{"duplicates", "/api/v1/storage-analyzer/reports/" + report.ID + "/duplicates"},
		{"big-files", "/api/v1/storage-analyzer/reports/" + report.ID + "/big-files"},
		{"suggestions", "/api/v1/storage-analyzer/reports/" + report.ID + "/suggestions"},
		{"heatmap", "/api/v1/storage-analyzer/reports/" + report.ID + "/heatmap"},
		{"snapshots", "/api/v1/storage-analyzer/reports/" + report.ID + "/snapshots"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", ep.path, nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for %s, got %d", ep.name, w.Code)
			}
		})
	}
}

func TestDuplicateDetection(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	report, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	// Should detect the duplicate files we created
	foundDuplicate := false
	for _, dup := range report.Duplicates {
		if dup.Count >= 2 {
			foundDuplicate = true
			break
		}
	}

	if !foundDuplicate {
		t.Error("Should have detected duplicate files")
	}
}

func TestFileTypeAnalysis(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	report, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	// Should have multiple file types
	if len(report.ByFileType) < 3 {
		t.Errorf("Expected at least 3 file types, got %d", len(report.ByFileType))
	}

	// Check that categories are assigned
	categories := make(map[string]bool)
	for _, ft := range report.ByFileType {
		if ft.Category != "" {
			categories[ft.Category] = true
		}
	}

	if len(categories) < 2 {
		t.Errorf("Expected at least 2 categories, got %d", len(categories))
	}
}

func TestCleanupSuggestions(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	report, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	// Suggestions should be generated (may be empty if files are small)
	if report.Suggestions == nil {
		t.Error("Suggestions should not be nil")
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	// Run analysis
	_, err := analyzer.RunAnalysis(ctx)
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}

	// Concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			analyzer.GetReports()
			analyzer.GetHistory()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHistoryRecording(t *testing.T) {
	tmpDir := createTestDir(t)

	config := AnalysisConfig{
		ScanPaths:     []string{tmpDir},
		ExcludePaths:  []string{},
		HashAlgorithm: "sha256",
	}

	analyzer := NewAnalyzer(config)
	ctx := context.Background()

	// Run analysis multiple times
	for i := 0; i < 3; i++ {
		_, err := analyzer.RunAnalysis(ctx)
		if err != nil {
			t.Fatalf("RunAnalysis failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Small delay for different timestamps
	}

	history := analyzer.GetHistory()
	if len(history) < 3 {
		t.Errorf("Expected at least 3 history entries, got %d", len(history))
	}

	// Verify timestamps are in order
	for i := 1; i < len(history); i++ {
		if history[i].Timestamp.Before(history[i-1].Timestamp) {
			t.Error("History timestamps should be in order")
		}
	}
}
