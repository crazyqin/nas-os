package spotlightcompat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	cfg := SpotlightConfig{
		Enabled:       true,
		IndexPath:     "/tmp/spotlight-test",
		MaxIndexSize:  1024,
		IndexInterval: 300,
		SMBShares:     []string{"/tmp"},
	}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.running {
		t.Error("manager should not be running initially")
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := m.Start(); err == nil {
		t.Error("double Start should fail")
	}
	status := m.GetStatus()
	if !status.Running {
		t.Error("expected running=true")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	status = m.GetStatus()
	if status.Running {
		t.Error("expected running=false")
	}
}

func TestSearch(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	m.index["test1"] = &SpotlightIndex{
		ID:       "test1",
		FileName: "report.pdf",
		FileType: "document",
		FullPath: "/share/report.pdf",
		Size:     1024,
	}
	m.index["test2"] = &SpotlightIndex{
		ID:       "test2",
		FileName: "photo.jpg",
		FileType: "image",
		FullPath: "/share/photo.jpg",
		Size:     2048,
	}

	resp := m.Search(SpotlightSearchRequest{Query: "report"})
	if resp.TotalCount != 1 {
		t.Errorf("expected 1 result, got %d", resp.TotalCount)
	}
	if len(resp.Results) > 0 && resp.Results[0].Index.ID != "test1" {
		t.Error("expected test1 as top result")
	}
}

func TestSearchPagination(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("file%d", i)
		m.index[id] = &SpotlightIndex{
			ID:       id,
			FileName: fmt.Sprintf("file%d.txt", i),
			FileType: "text",
			FullPath: fmt.Sprintf("/share/file%d.txt", i),
		}
	}
	resp := m.Search(SpotlightSearchRequest{Query: "file", Page: 1, PageSize: 10})
	if len(resp.Results) != 10 {
		t.Errorf("expected 10 results, got %d", len(resp.Results))
	}
	if resp.TotalCount != 50 {
		t.Errorf("expected total 50, got %d", resp.TotalCount)
	}
}

func TestSearchFilter(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	m.index["a"] = &SpotlightIndex{ID: "a", FileName: "a.jpg", FileType: "image", FullPath: "/a.jpg"}
	m.index["b"] = &SpotlightIndex{ID: "b", FileName: "b.pdf", FileType: "document", FullPath: "/b.pdf"}

	resp := m.Search(SpotlightSearchRequest{Query: "", FileType: "image"})
	if resp.TotalCount != 1 {
		t.Errorf("expected 1 image, got %d", resp.TotalCount)
	}
}

func TestIndexDirectory(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	task, err := m.IndexDirectory("/tmp")
	if err != nil {
		t.Fatalf("IndexDirectory failed: %v", err)
	}
	if task.Status != "running" && task.Status != "completed" {
		t.Errorf("unexpected task status: %s", task.Status)
	}
}

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"photo.jpg", "image"},
		{"video.mp4", "video"},
		{"song.mp3", "audio"},
		{"doc.pdf", "document"},
		{"readme.md", "text"},
		{"main.go", "code"},
		{"archive.zip", "archive"},
		{"unknown.xyz", "other"},
	}
	for _, tt := range tests {
		got := classifyFile(tt.name)
		if got != tt.want {
			t.Errorf("classifyFile(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestHandlerSearch(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	m.index["t1"] = &SpotlightIndex{ID: "t1", FileName: "test.txt", FileType: "text", FullPath: "/test.txt"}
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"query":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/spotlight/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandlerStatus(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	h := NewHandler(m)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/spotlight/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetConfig(t *testing.T) {
	cfg := SpotlightConfig{Enabled: true, IndexPath: "/idx", SMBShares: []string{"/s1"}}
	m := NewManager(cfg)
	got := m.GetConfig()
	if got.IndexPath != "/idx" {
		t.Errorf("expected /idx, got %s", got.IndexPath)
	}
}

func TestUpdateConfig(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	newCfg := SpotlightConfig{Enabled: true, IndexPath: "/new", SMBShares: []string{"/new"}}
	if err := m.UpdateConfig(newCfg); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	got := m.GetConfig()
	if got.IndexPath != "/new" {
		t.Errorf("expected /new, got %s", got.IndexPath)
	}
}

func TestRemoveFromIndex(t *testing.T) {
	m := NewManager(SpotlightConfig{SMBShares: []string{"/tmp"}})
	m.index["rm1"] = &SpotlightIndex{ID: "rm1", FileName: "rm.txt"}
	if err := m.RemoveFromIndex("rm1"); err != nil {
		t.Fatalf("RemoveFromIndex failed: %v", err)
	}
	if _, ok := m.index["rm1"]; ok {
		t.Error("entry should have been removed")
	}
	if err := m.RemoveFromIndex("nonexistent"); err == nil {
		t.Error("expected error for nonexistent entry")
	}
}
