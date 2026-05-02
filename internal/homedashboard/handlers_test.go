// Package homedashboard HTTP handler 测试
package homedashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestHandlers() *Handlers {
	m := NewManager()
	return NewHandlers(m)
}

func TestHandlersCreateDashboard(t *testing.T) {
	h := setupTestHandlers()
	body := `{"user_id":"u1","name":"我的仪表盘"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleDashboards(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("状态码应为201: %d", w.Code)
	}

	var resp apiResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 0 {
		t.Errorf("响应码应为0: %d", resp.Code)
	}
}

func TestHandlersCreateDashboardMissingUserID(t *testing.T) {
	h := setupTestHandlers()
	body := `{"name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleDashboards(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("状态码应为400: %d", w.Code)
	}
}

func TestHandlersCreateDashboardDefaultName(t *testing.T) {
	h := setupTestHandlers()
	body := `{"user_id":"u1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.handleDashboards(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("状态码应为201: %d", w.Code)
	}

	var resp apiResponse
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp.Data.(map[string]interface{})
	if data["name"] != "我的仪表盘" {
		t.Errorf("默认名称不匹配: %v", data["name"])
	}
}

func TestHandlersListDashboards(t *testing.T) {
	h := setupTestHandlers()

	// 创建仪表盘
	body := `{"user_id":"u1","name":"d1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	// 列出仪表盘
	req = httptest.NewRequest(http.MethodGet, "/api/v1/homedashboard/dashboards", nil)
	w = httptest.NewRecorder()
	h.handleDashboards(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}

	var resp apiResponse
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp.Data.(map[string]interface{})
	if int(data["total"].(float64)) != 1 {
		t.Errorf("应有1个仪表盘: %v", data["total"])
	}
}

func TestHandlersGetDashboard(t *testing.T) {
	h := setupTestHandlers()

	// 创建
	body := `{"user_id":"u1","name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	dashboardID := createResp.Data.(map[string]interface{})["id"].(string)

	// 获取
	req = httptest.NewRequest(http.MethodGet, "/api/v1/homedashboard/dashboards/"+dashboardID, nil)
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersGetDashboardNotFound(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/homedashboard/dashboards/nonexistent", nil)
	w := httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("状态码应为404: %d", w.Code)
	}
}

func TestHandlersUpdateDashboard(t *testing.T) {
	h := setupTestHandlers()

	// 创建
	body := `{"user_id":"u1","name":"old"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	dashboardID := createResp.Data.(map[string]interface{})["id"].(string)

	// 更新
	updateBody := `{"name":"new"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/homedashboard/dashboards/"+dashboardID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersDeleteDashboard(t *testing.T) {
	h := setupTestHandlers()

	// 创建
	body := `{"user_id":"u1","name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	dashboardID := createResp.Data.(map[string]interface{})["id"].(string)

	// 删除
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/homedashboard/dashboards/"+dashboardID, nil)
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersAddLayout(t *testing.T) {
	h := setupTestHandlers()

	// 创建仪表盘
	body := `{"user_id":"u1","name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	dashboardID := createResp.Data.(map[string]interface{})["id"].(string)

	// 添加布局
	layoutBody := `{"name":"布局2","columns":8,"rows":6}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards/"+dashboardID+"/layouts", bytes.NewBufferString(layoutBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("状态码应为201: %d", w.Code)
	}
}

func TestHandlersAddWidget(t *testing.T) {
	h := setupTestHandlers()

	// 创建仪表盘
	body := `{"user_id":"u1","name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	dashboardData := createResp.Data.(map[string]interface{})
	dashboardID := dashboardData["id"].(string)
	layouts := dashboardData["layouts"].([]interface{})
	layoutData := layouts[0].(map[string]interface{})
	layoutID := layoutData["id"].(string)

	// 添加Widget
	widgetBody := `{"type":"weather","title":"天气","size":{"width":4,"height":3}}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards/"+dashboardID+"/layouts/"+layoutID+"/widgets", bytes.NewBufferString(widgetBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("状态码应为201: %d", w.Code)
	}
}

func TestHandlersGetWidget(t *testing.T) {
	h := setupTestHandlers()

	// 创建仪表盘
	body := `{"user_id":"u1","name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	dashboardData := createResp.Data.(map[string]interface{})
	dashboardID := dashboardData["id"].(string)
	layouts := dashboardData["layouts"].([]interface{})
	layoutData := layouts[0].(map[string]interface{})
	layoutID := layoutData["id"].(string)

	// 添加Widget
	widgetBody := `{"type":"weather","title":"天气"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards/"+dashboardID+"/layouts/"+layoutID+"/widgets", bytes.NewBufferString(widgetBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	var widgetResp apiResponse
	json.NewDecoder(w.Body).Decode(&widgetResp)
	widgetID := widgetResp.Data.(map[string]interface{})["id"].(string)

	// 获取Widget
	req = httptest.NewRequest(http.MethodGet, "/api/v1/homedashboard/dashboards/"+dashboardID+"/layouts/"+layoutID+"/widgets/"+widgetID, nil)
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersUpdateWidget(t *testing.T) {
	h := setupTestHandlers()

	// 创建仪表盘
	body := `{"user_id":"u1","name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	dashboardData := createResp.Data.(map[string]interface{})
	dashboardID := dashboardData["id"].(string)
	layouts := dashboardData["layouts"].([]interface{})
	layoutData := layouts[0].(map[string]interface{})
	layoutID := layoutData["id"].(string)

	// 添加Widget
	widgetBody := `{"type":"weather","title":"天气"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards/"+dashboardID+"/layouts/"+layoutID+"/widgets", bytes.NewBufferString(widgetBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	var widgetResp apiResponse
	json.NewDecoder(w.Body).Decode(&widgetResp)
	widgetID := widgetResp.Data.(map[string]interface{})["id"].(string)

	// 更新Widget
	updateBody := `{"title":"新天气"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/homedashboard/dashboards/"+dashboardID+"/layouts/"+layoutID+"/widgets/"+widgetID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersDeleteWidget(t *testing.T) {
	h := setupTestHandlers()

	// 创建仪表盘
	body := `{"user_id":"u1","name":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	var createResp apiResponse
	json.NewDecoder(w.Body).Decode(&createResp)
	dashboardData := createResp.Data.(map[string]interface{})
	dashboardID := dashboardData["id"].(string)
	layouts := dashboardData["layouts"].([]interface{})
	layoutData := layouts[0].(map[string]interface{})
	layoutID := layoutData["id"].(string)

	// 添加Widget
	widgetBody := `{"type":"weather","title":"天气"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards/"+dashboardID+"/layouts/"+layoutID+"/widgets", bytes.NewBufferString(widgetBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	var widgetResp apiResponse
	json.NewDecoder(w.Body).Decode(&widgetResp)
	widgetID := widgetResp.Data.(map[string]interface{})["id"].(string)

	// 删除Widget
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/homedashboard/dashboards/"+dashboardID+"/layouts/"+layoutID+"/widgets/"+widgetID, nil)
	w = httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersListTemplates(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/homedashboard/templates", nil)
	w := httptest.NewRecorder()

	h.handleTemplates(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersGetTemplate(t *testing.T) {
	h := setupTestHandlers()

	// 注册模板
	h.manager.RegisterTemplate(&WidgetTemplate{ID: "tmpl-1", Name: "test"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/homedashboard/templates/tmpl-1", nil)
	w := httptest.NewRecorder()
	h.handleTemplateByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersGetTemplateNotFound(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/homedashboard/templates/nonexistent", nil)
	w := httptest.NewRecorder()
	h.handleTemplateByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("状态码应为404: %d", w.Code)
	}
}

func TestHandlersDownloadTemplate(t *testing.T) {
	h := setupTestHandlers()
	h.manager.RegisterTemplate(&WidgetTemplate{ID: "tmpl-1", Name: "test", Downloads: 0})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/templates/tmpl-1/download", nil)
	w := httptest.NewRecorder()
	h.handleTemplateByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersRateTemplate(t *testing.T) {
	h := setupTestHandlers()
	h.manager.RegisterTemplate(&WidgetTemplate{ID: "tmpl-1", Name: "test"})

	body := `{"rating":4.5}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/templates/tmpl-1/rate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleTemplateByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200: %d", w.Code)
	}
}

func TestHandlersMethodNotAllowed(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/homedashboard/dashboards", nil)
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("状态码应为405: %d", w.Code)
	}
}

func TestHandlersDashboardByIDEmpty(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/homedashboard/dashboards/", nil)
	w := httptest.NewRecorder()
	h.handleDashboardByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("空路径应返回404: %d", w.Code)
	}
}

func TestHandlersInvalidJSON(t *testing.T) {
	h := setupTestHandlers()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/homedashboard/dashboards", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleDashboards(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("无效JSON应返回400: %d", w.Code)
	}
}

func TestHandlersRegisterRoutes(t *testing.T) {
	h := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	if mux == nil {
		t.Error("mux不应为nil")
	}
}
