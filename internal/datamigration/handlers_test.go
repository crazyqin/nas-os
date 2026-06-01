package datamigration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	m := NewManager("/tmp/test_migration.json")
	m.Initialize()
	handler := NewHandler(m)
	r := gin.New()
	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)
	return r, m
}

func TestListMigrationsAPI(t *testing.T) {
	r, m := setupTestRouter()

	m.CreateMigration(CreateMigrationRequest{
		Name: "任务1", SourceType: "local",
		Source: Source{Type: "local", Path: "/a"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/b"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-migration/migrations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp["success"].(bool) {
		t.Error("Expected success=true")
	}
}

func TestListMigrationsWithStatusFilter(t *testing.T) {
	r, m := setupTestRouter()

	mig, _ := m.CreateMigration(CreateMigrationRequest{
		Name: "任务1", SourceType: "local",
		Source: Source{Type: "local", Path: "/a"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/b"},
	})
	m.StartMigration(mig.ID)

	m.CreateMigration(CreateMigrationRequest{
		Name: "任务2", SourceType: "local",
		Source: Source{Type: "local", Path: "/c"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/d"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-migration/migrations?status=running", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("Expected 1 running task, got %d", len(data))
	}
}

func TestCreateMigrationAPI(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateMigrationRequest{
		Name:       "API创建",
		SourceType: "local",
		Source:     Source{Type: "local", Path: "/source"},
		TargetType: "local",
		Target:     Target{Type: "local", Path: "/target"},
		Options:    MigrationOptions{Parallel: 4},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/data-migration/migrations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateMigrationInvalidJSON(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/data-migration/migrations", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetMigrationAPI(t *testing.T) {
	r, m := setupTestRouter()

	mig, _ := m.CreateMigration(CreateMigrationRequest{
		Name: "测试", SourceType: "local",
		Source: Source{Type: "local", Path: "/a"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/b"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-migration/migrations/"+mig.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetMigrationNotFound(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-migration/migrations/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestStartMigrationAPI(t *testing.T) {
	r, m := setupTestRouter()

	mig, _ := m.CreateMigration(CreateMigrationRequest{
		Name: "测试", SourceType: "local",
		Source: Source{Type: "local", Path: "/a"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/b"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/data-migration/migrations/"+mig.ID+"/start", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	updated, _ := m.GetMigration(mig.ID)
	if updated.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", updated.Status)
	}
}

func TestPauseMigrationAPI(t *testing.T) {
	r, m := setupTestRouter()

	mig, _ := m.CreateMigration(CreateMigrationRequest{
		Name: "测试", SourceType: "local",
		Source: Source{Type: "local", Path: "/a"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/b"},
	})
	m.StartMigration(mig.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/data-migration/migrations/"+mig.ID+"/pause", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	updated, _ := m.GetMigration(mig.ID)
	if updated.Status != "paused" {
		t.Errorf("Expected status 'paused', got '%s'", updated.Status)
	}
}

func TestCancelMigrationAPI(t *testing.T) {
	r, m := setupTestRouter()

	mig, _ := m.CreateMigration(CreateMigrationRequest{
		Name: "测试", SourceType: "local",
		Source: Source{Type: "local", Path: "/a"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/b"},
	})
	m.StartMigration(mig.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/data-migration/migrations/"+mig.ID+"/cancel", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	updated, _ := m.GetMigration(mig.ID)
	if updated.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got '%s'", updated.Status)
	}
}

func TestGetProgressAPI(t *testing.T) {
	r, m := setupTestRouter()

	mig, _ := m.CreateMigration(CreateMigrationRequest{
		Name: "测试", SourceType: "local",
		Source: Source{Type: "local", Path: "/a"}, TargetType: "local",
		Target: Target{Type: "local", Path: "/b"},
	})
	m.StartMigration(mig.ID)
	m.UpdateProgress(mig.ID, Progress{
		TotalFiles:     100,
		CompletedFiles: 75,
		TotalBytes:     1000000,
		CompletedBytes: 750000,
		Speed:          50000,
		CurrentFile:    "file75.txt",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-migration/migrations/"+mig.ID+"/progress", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["progress"] == nil {
		t.Error("Expected progress data")
	}
}

func TestListSourcesAPI(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-migration/sources", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) < 5 {
		t.Errorf("Expected at least 5 source types, got %d", len(data))
	}
}

func TestListTargetsAPI(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-migration/targets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) < 3 {
		t.Errorf("Expected at least 3 target types, got %d", len(data))
	}
}
