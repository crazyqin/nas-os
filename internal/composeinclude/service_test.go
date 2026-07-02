package composeinclude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSimpleCompose(t *testing.T) {
	m := NewIncludeManager()
	content := `{
		"services": {
			"web": {
				"image": "nginx:latest",
				"ports": ["80:80"]
			}
		}
	}`
	req := &ParseRequest{Content: content}
	result, err := m.Parse(req)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if result.ServiceCount != 1 {
		t.Errorf("expected 1 service, got %d", result.ServiceCount)
	}
	if _, ok := result.MergedServices["web"]; !ok {
		t.Error("expected 'web' service in merged services")
	}
	if len(result.IncludePaths) != 0 {
		t.Errorf("expected 0 include paths, got %d", len(result.IncludePaths))
	}
	if !result.AllFilesExist {
		t.Error("expected AllFilesExist to be true with no includes")
	}
}

func TestParseNilRequest(t *testing.T) {
	m := NewIncludeManager()
	if _, err := m.Parse(nil); err == nil {
		t.Error("expected error for nil request")
	}
}

func TestParseEmptyContent(t *testing.T) {
	m := NewIncludeManager()
	req := &ParseRequest{Content: ""}
	if _, err := m.Parse(req); err == nil {
		t.Error("expected error for empty content")
	}
}

func TestParseInvalidJSON(t *testing.T) {
	m := NewIncludeManager()
	req := &ParseRequest{Content: "not json"}
	if _, err := m.Parse(req); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseWithIncludeMissingFiles(t *testing.T) {
	m := NewIncludeManager()
	content := `{
		"services": {
			"app": {
				"image": "myapp:latest"
			}
		},
		"include": [
			{
				"paths": ["./nonexistent.yml"]
			}
		]
	}`
	req := &ParseRequest{Content: content, BaseDir: "/tmp"}
	result, err := m.Parse(req)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if result.AllFilesExist {
		t.Error("expected AllFilesExist to be false")
	}
	if len(result.MissingFiles) == 0 {
		t.Error("expected missing files to be reported")
	}
	if len(result.IncludePaths) != 1 {
		t.Errorf("expected 1 include path, got %d", len(result.IncludePaths))
	}
}

func TestParseWithIncludeExistingFiles(t *testing.T) {
	m := NewIncludeManager()
	// 创建临时外部 Compose 文件
	tmpDir := t.TempDir()
	externalContent := `{
		"services": {
			"db": {
				"image": "postgres:15",
				"ports": ["5432:5432"]
			}
		}
	}`
	externalPath := filepath.Join(tmpDir, "db.yml")
	if err := os.WriteFile(externalPath, []byte(externalContent), 0644); err != nil {
		t.Fatalf("failed to write external file: %v", err)
	}

	content := `{
		"services": {
			"web": {
				"image": "nginx:latest"
			}
		},
		"include": [
			{
				"paths": ["db.yml"]
			}
		]
	}`
	req := &ParseRequest{Content: content, BaseDir: tmpDir}
	result, err := m.Parse(req)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if !result.AllFilesExist {
		t.Error("expected AllFilesExist to be true")
	}
	if result.ServiceCount != 2 {
		t.Errorf("expected 2 services after merge, got %d", result.ServiceCount)
	}
	if _, ok := result.MergedServices["web"]; !ok {
		t.Error("expected 'web' service in merged services")
	}
	if _, ok := result.MergedServices["db"]; !ok {
		t.Error("expected 'db' service from include to be merged")
	}
}

func TestParseIncludeDoesNotOverrideExisting(t *testing.T) {
	m := NewIncludeManager()
	tmpDir := t.TempDir()
	externalContent := `{
		"services": {
			"web": {
				"image": "nginx:external"
			}
		}
	}`
	externalPath := filepath.Join(tmpDir, "external.yml")
	os.WriteFile(externalPath, []byte(externalContent), 0644)

	content := `{
		"services": {
			"web": {
				"image": "nginx:main"
			}
		},
		"include": [
			{
				"paths": ["external.yml"]
			}
		]
	}`
	req := &ParseRequest{Content: content, BaseDir: tmpDir}
	result, err := m.Parse(req)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	svc := result.MergedServices["web"]
	if svc.Image != "nginx:main" {
		t.Errorf("expected main file to take precedence, got image %q", svc.Image)
	}
}

func TestGetResult(t *testing.T) {
	m := NewIncludeManager()
	content := `{"services": {"app": {"image": "app:latest"}}}`
	result, _ := m.Parse(&ParseRequest{Content: content})

	got, err := m.GetResult(result.ID)
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if got.ID != result.ID {
		t.Errorf("expected ID %q, got %q", result.ID, got.ID)
	}
}

func TestGetResultNotFound(t *testing.T) {
	m := NewIncludeManager()
	if _, err := m.GetResult("nonexistent"); err == nil {
		t.Error("expected error for nonexistent result")
	}
}

func TestListResults(t *testing.T) {
	m := NewIncludeManager()
	m.Parse(&ParseRequest{Content: `{"services": {"a": {"image": "a"}}}`})
	m.Parse(&ParseRequest{Content: `{"services": {"b": {"image": "b"}}}`})

	results := m.ListResults()
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestListResultsEmpty(t *testing.T) {
	m := NewIncludeManager()
	results := m.ListResults()
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRemoveResult(t *testing.T) {
	m := NewIncludeManager()
	result, _ := m.Parse(&ParseRequest{Content: `{"services": {"a": {"image": "a"}}}`})
	m.RemoveResult(result.ID)
	if _, err := m.GetResult(result.ID); err == nil {
		t.Error("expected error after remove")
	}
}

func TestValidateIncludePaths(t *testing.T) {
	m := NewIncludeManager()
	tmpDir := t.TempDir()
	existingPath := filepath.Join(tmpDir, "exists.yml")
	os.WriteFile(existingPath, []byte("{}"), 0644)

	paths := []string{"exists.yml", "nonexistent.yml"}
	existing, missing := m.ValidateIncludePaths(paths, tmpDir)

	if len(existing) != 1 {
		t.Errorf("expected 1 existing file, got %d", len(existing))
	}
	if len(missing) != 1 {
		t.Errorf("expected 1 missing file, got %d", len(missing))
	}
}

func TestValidateIncludePathsAllExist(t *testing.T) {
	m := NewIncludeManager()
	tmpDir := t.TempDir()
	p1 := filepath.Join(tmpDir, "a.yml")
	p2 := filepath.Join(tmpDir, "b.yml")
	os.WriteFile(p1, []byte("{}"), 0644)
	os.WriteFile(p2, []byte("{}"), 0644)

	paths := []string{"a.yml", "b.yml"}
	existing, missing := m.ValidateIncludePaths(paths, tmpDir)

	if len(existing) != 2 {
		t.Errorf("expected 2 existing, got %d", len(existing))
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing, got %d", len(missing))
	}
}

func TestMergeServices(t *testing.T) {
	m := NewIncludeManager()
	base := map[string]ServiceDefinition{
		"web": {Image: "nginx:latest"},
		"db":  {Image: "postgres:15"},
	}
	overlay := map[string]ServiceDefinition{
		"cache": {Image: "redis:7"},
		"db":    {Image: "postgres:16"},
	}

	merged := m.MergeServices(base, overlay)
	if len(merged) != 3 {
		t.Errorf("expected 3 services, got %d", len(merged))
	}
	if merged["db"].Image != "postgres:16" {
		t.Errorf("expected overlay to override base, got %q", merged["db"].Image)
	}
}

func TestParseResultIDUniqueness(t *testing.T) {
	m := NewIncludeManager()
	r1, _ := m.Parse(&ParseRequest{Content: `{"services": {"a": {}}}`})
	r2, _ := m.Parse(&ParseRequest{Content: `{"services": {"b": {}}}`})
	if r1.ID == r2.ID {
		t.Error("expected unique IDs for different parse results")
	}
}
