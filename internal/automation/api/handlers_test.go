package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nas-os/internal/automation/engine"

	"github.com/gorilla/mux"
)

func TestNewAutomationAPI(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)
	if api == nil {
		t.Fatal("NewAutomationAPI should not return nil")
	}
}

func TestListWorkflows(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)

	router := mux.NewRouter()
	api.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/automation/workflows", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)

	router := mux.NewRouter()
	api.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/automation/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	// 验证统计字段存在
	if _, ok := resp["total_workflows"]; !ok {
		t.Error("Expected total_workflows field in stats")
	}
}

func TestGetWorkflowNotFound(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)

	router := mux.NewRouter()
	api.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/automation/workflows/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestDeleteWorkflowNotFound(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)

	router := mux.NewRouter()
	api.RegisterRoutes(router)

	req := httptest.NewRequest("DELETE", "/api/automation/workflows/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestListTemplates(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)

	router := mux.NewRouter()
	api.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/automation/templates", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestListTemplateCategories(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)

	router := mux.NewRouter()
	api.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/automation/templates/categories", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetTemplateNotFound(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)

	router := mux.NewRouter()
	api.RegisterRoutes(router)

	req := httptest.NewRequest("GET", "/api/automation/templates/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestCreateWorkflowInvalidJSON(t *testing.T) {
	eng := engine.NewWorkflowEngine()
	api := NewAutomationAPI(eng)

	router := mux.NewRouter()
	api.RegisterRoutes(router)

	req := httptest.NewRequest("POST", "/api/automation/workflows", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
