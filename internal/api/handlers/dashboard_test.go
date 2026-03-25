// Package handlers 提供 Dashboard API 端点测试
// v2.56.0 - 兵部实现
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nas-os/internal/dashboard"
	"nas-os/internal/monitor"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestHandlers(t *testing.T) (*DashboardHandlers, *gin.Engine) {
	monitorMgr, _ := monitor.NewManager()
	dashboardMgr, _ := dashboard.NewManager(&dashboard.ManagerConfig{
		DataDir:        t.TempDir(),
		MonitorManager: monitorMgr,
	})

	handlers := NewDashboardHandlers(dashboardMgr, monitorMgr, nil)

	router := gin.New()
	api := router.Group("/api/v1")
	handlers.RegisterRoutes(api)

	return handlers, router
}

func TestListDashboards(t *testing.T) {
	_, router := setupTestHandlers(t)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboards", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["code"].(float64) != 0 {
		t.Errorf("Expected code 0, got %v", resp["code"])
	}
}

func TestCreateDashboard(t *testing.T) {
	_, router := setupTestHandlers(t)

	body := map[string]string{
		"name":        "Test Dashboard",
		"description": "Test Description",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboards", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["code"].(float64) != 0 {
		t.Errorf("Expected code 0, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if data["name"] != "Test Dashboard" {
		t.Errorf("Expected name 'Test Dashboard', got %v", data["name"])
	}
}

func TestGetDashboard(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	// 先创建一个仪表板
	d, _ := handlers.manager.CreateDashboard("Test", "Test Description")

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboards/"+d.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	if data["id"] != d.ID {
		t.Errorf("Expected ID %s, got %v", d.ID, data["id"])
	}
}

func TestUpdateDashboard(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	d, _ := handlers.manager.CreateDashboard("Original", "Original Description")

	body := map[string]string{
		"name":        "Updated Dashboard",
		"description": "Updated Description",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/dashboards/"+d.ID, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	if data["name"] != "Updated Dashboard" {
		t.Errorf("Expected name 'Updated Dashboard', got %v", data["name"])
	}
}

func TestDeleteDashboard(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	d, _ := handlers.manager.CreateDashboard("To Delete", "Will be deleted")

	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/dashboards/"+d.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// 验证已删除
	_, err := handlers.manager.GetDashboard(d.ID)
	if err == nil {
		t.Error("Expected dashboard to be deleted")
	}
}

func TestAddWidget(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	d, _ := handlers.manager.CreateDashboard("Test", "Test")

	body := map[string]interface{}{
		"type":        "cpu",
		"title":       "CPU Monitor",
		"size":        "medium",
		"position":    map[string]int{"x": 0, "y": 0},
		"refreshRate": 5,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboards/"+d.ID+"/widgets", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["code"].(float64) != 0 {
		t.Errorf("Expected code 0, got %v", resp["code"])
	}
}

func TestUpdateWidgetPosition(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	d, _ := handlers.manager.CreateDashboard("Test", "Test")
	widget := &dashboard.Widget{
		Type:        dashboard.WidgetTypeCPU,
		Title:       "CPU",
		Size:        dashboard.WidgetSizeMedium,
		Position:    dashboard.WidgetPosition{X: 0, Y: 0},
		Enabled:     true,
		RefreshRate: 5 * time.Second,
	}
	handlers.manager.AddWidget(d.ID, widget)

	body := map[string]int{"x": 1, "y": 2}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/dashboards/"+d.ID+"/widgets/"+widget.ID+"/position", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// 验证位置已更新
	updated, _ := handlers.manager.GetDashboard(d.ID)
	for _, w := range updated.Widgets {
		if w.ID == widget.ID {
			if w.Position.X != 1 || w.Position.Y != 2 {
				t.Errorf("Expected position (1, 2), got (%d, %d)", w.Position.X, w.Position.Y)
			}
		}
	}
}

func TestGetWidgetTypes(t *testing.T) {
	_, router := setupTestHandlers(t)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/widgets/types", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].([]interface{})
	if len(data) < 4 {
		t.Errorf("Expected at least 4 widget types, got %d", len(data))
	}
}

func TestGetTemplates(t *testing.T) {
	_, router := setupTestHandlers(t)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboards/templates", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].([]interface{})
	if len(data) < 3 {
		t.Errorf("Expected at least 3 templates, got %d", len(data))
	}
}

func TestCreateFromTemplate(t *testing.T) {
	_, router := setupTestHandlers(t)

	body := map[string]string{
		"templateId": "system-monitor",
		"name":       "My Monitor",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboards/from-template", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	if data["name"] != "My Monitor" {
		t.Errorf("Expected name 'My Monitor', got %v", data["name"])
	}
}

func TestGetDefaultDashboard(t *testing.T) {
	_, router := setupTestHandlers(t)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/dashboard/default", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	if data["isDefault"].(bool) != true {
		t.Error("Expected default dashboard")
	}
}

func TestResetLayout(t *testing.T) {
	handlers, router := setupTestHandlers(t)

	d, _ := handlers.manager.CreateDashboard("Test", "Test")

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/dashboards/"+d.ID+"/layout/reset", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["code"].(float64) != 0 {
		t.Errorf("Expected code 0, got %v", resp["code"])
	}
}
