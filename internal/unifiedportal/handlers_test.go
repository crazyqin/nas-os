// Package unifiedportal 提供单元测试
package unifiedportal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *PortalManager) {
	gin.SetMode(gin.TestMode)
	pm := NewPortalManager(nil)
	h := NewHandlers(pm)

	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)

	return r, pm
}

// TestCreateDashboard 测试创建仪表盘
func TestCreateDashboard(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := DashboardRequest{
		Name:        "测试仪表盘",
		Description: "这是一个测试仪表盘",
		Layout:      LayoutGrid,
		Tags:        []string{"test"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/dashboards", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "测试仪表盘", data["name"])
	assert.Equal(t, "grid", data["layout"])
}

// TestListDashboards 测试列出仪表盘
func TestListDashboards(t *testing.T) {
	r, pm := setupTestRouter()

	// 先创建一个仪表盘
	pm.CreateDashboard(&DashboardRequest{
		Name:   "仪表盘1",
		Layout: LayoutGrid,
	}, "user1")
	pm.CreateDashboard(&DashboardRequest{
		Name:   "仪表盘2",
		Layout: LayoutResponsive,
	}, "user2")

	req := httptest.NewRequest(http.MethodGet, "/api/portal/dashboards", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	// 应该包含3个模板 + 2个新建的
	assert.GreaterOrEqual(t, len(data), 5)
}

// TestAddWidget 测试添加Widget
func TestAddWidget(t *testing.T) {
	r, pm := setupTestRouter()

	// 先创建仪表盘
	dashboard, _ := pm.CreateDashboard(&DashboardRequest{
		Name:   "测试仪表盘",
		Layout: LayoutGrid,
	}, "user1")

	reqBody := WidgetRequest{
		Type:  WidgetSystemOverview,
		Title: "系统概览",
		Position: WidgetPosition{X: 0, Y: 0},
		Size:     WidgetSize{Width: 6, Height: 4},
		RefreshSec: 30,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/dashboards/"+dashboard.ID+"/widgets", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "system_overview", data["type"])
	assert.Equal(t, "系统概览", data["title"])
}

// TestSwitchTheme 测试切换主题
func TestSwitchTheme(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := ThemeRequest{
		ThemeID: "theme-dark",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPut, "/api/portal/theme", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "暗黑主题", data["name"])
	assert.Equal(t, true, data["is_dark"])
}

// TestDeleteDashboard 测试删除仪表盘
func TestDeleteDashboard(t *testing.T) {
	r, pm := setupTestRouter()

	// 创建仪表盘
	dashboard, _ := pm.CreateDashboard(&DashboardRequest{
		Name:   "待删除仪表盘",
		Layout: LayoutGrid,
	}, "user1")

	req := httptest.NewRequest(http.MethodDelete, "/api/portal/dashboards/"+dashboard.ID, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证已删除
	_, err := pm.GetDashboard(dashboard.ID)
	assert.Error(t, err)
}

// TestGetMetrics 测试获取聚合指标
func TestGetMetrics(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/portal/metrics", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.NotNil(t, data["system"])
	assert.NotNil(t, data["storage"])
	assert.NotNil(t, data["network"])
	assert.NotNil(t, data["container"])
	assert.NotNil(t, data["alerts"])
}

// TestCreateDashboardFromTemplate 测试从模板创建仪表盘
func TestCreateDashboardFromTemplate(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/portal/dashboards/from-template/template-admin?name=我的管理面板", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "我的管理面板", data["name"])
	assert.Equal(t, "template-admin", data["template_id"])
}

// TestWidgetCollision 测试Widget位置冲突检测
func TestWidgetCollision(t *testing.T) {
	r, pm := setupTestRouter()

	// 创建仪表盘
	dashboard, _ := pm.CreateDashboard(&DashboardRequest{
		Name:   "冲突测试",
		Layout: LayoutGrid,
	}, "user1")

	// 添加第一个Widget
	pm.AddWidget(dashboard.ID, &WidgetRequest{
		Type:     WidgetSystemOverview,
		Title:    "Widget1",
		Position: WidgetPosition{X: 0, Y: 0},
		Size:     WidgetSize{Width: 6, Height: 4},
	})

	// 尝试添加位置冲突的Widget
	reqBody := WidgetRequest{
		Type:  WidgetStorageUsage,
		Title: "Widget2",
		Position: WidgetPosition{X: 3, Y: 2},
		Size:     WidgetSize{Width: 6, Height: 4},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/portal/dashboards/"+dashboard.ID+"/widgets", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 应该返回错误
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 1, resp.Code)
}

// TestGetThemes 测试获取主题列表
func TestGetThemes(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/portal/themes", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.GreaterOrEqual(t, len(data), 3) // 至少3个默认主题
}

// TestWidgetMove 测试Widget移动
func TestWidgetMove(t *testing.T) {
	r, pm := setupTestRouter()

	// 创建仪表盘和Widget
	dashboard, _ := pm.CreateDashboard(&DashboardRequest{
		Name:   "移动测试",
		Layout: LayoutGrid,
	}, "user1")

	widget, _ := pm.AddWidget(dashboard.ID, &WidgetRequest{
		Type:     WidgetSystemOverview,
		Title:    "可移动Widget",
		Position: WidgetPosition{X: 0, Y: 0},
		Size:     WidgetSize{Width: 6, Height: 4},
	})

	reqBody := WidgetMoveRequest{
		Position: WidgetPosition{X: 6, Y: 4},
		Size:     WidgetSize{Width: 4, Height: 3},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/api/portal/widgets/"+widget.ID+"/move", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)

	pos := data["position"].(map[string]interface{})
	assert.Equal(t, float64(6), pos["x"])
	assert.Equal(t, float64(4), pos["y"])
}

// TestSearchDashboards 测试搜索仪表盘
func TestSearchDashboards(t *testing.T) {
	r, pm := setupTestRouter()

	// 创建测试仪表盘
	pm.CreateDashboard(&DashboardRequest{
		Name: "生产环境监控",
		Description: "生产环境系统监控",
		Layout: LayoutGrid,
		Tags: []string{"prod"},
	}, "user1")

	// 按关键词搜索
	req := httptest.NewRequest(http.MethodGet, "/api/portal/dashboards/search?q=生产", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 1, len(data))

	// 按标签搜索
	req = httptest.NewRequest(http.MethodGet, "/api/portal/dashboards/search?tag=prod", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	data, ok = resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 1, len(data))

	// 仅模板搜索
	req = httptest.NewRequest(http.MethodGet, "/api/portal/dashboards/search?only_templates=true", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	data, ok = resp.Data.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 3, len(data)) // 3个默认模板
}

// TestDuplicateDashboard 测试复制仪表盘
func TestDuplicateDashboard(t *testing.T) {
	r, pm := setupTestRouter()

	// 创建原始仪表盘
	original, _ := pm.CreateDashboard(&DashboardRequest{
		Name: "原始仪表盘",
		Description: "用于测试复制",
		Layout: LayoutGrid,
		Tags: []string{"test"},
	}, "user1")

	// 添加Widget
	pm.AddWidget(original.ID, &WidgetRequest{
		Type:  WidgetSystemOverview,
		Title: "系统概览",
		Position: WidgetPosition{X: 0, Y: 0},
		Size:     WidgetSize{Width: 6, Height: 4},
	})

	// 复制仪表盘
	req := httptest.NewRequest(http.MethodPost, "/api/portal/dashboards/"+original.ID+"/duplicate?name=副本", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "副本", data["name"])
	assert.NotEqual(t, original.ID, data["id"])

	// 验证widget也被复制了
	widgets, ok := data["widgets"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 1, len(widgets))
}

// TestToggleWidgetVisibility 测试Widget可见性切换
func TestToggleWidgetVisibility(t *testing.T) {
	r, pm := setupTestRouter()

	// 创建仪表盘和Widget
	dashboard, _ := pm.CreateDashboard(&DashboardRequest{
		Name: "可见性测试",
		Layout: LayoutGrid,
	}, "user1")

	widget, _ := pm.AddWidget(dashboard.ID, &WidgetRequest{
		Type:  WidgetSystemOverview,
		Title: "隐藏测试",
		Position: WidgetPosition{X: 0, Y: 0},
		Size:     WidgetSize{Width: 6, Height: 4},
	})

	// 切换为隐藏
	reqBody := map[string]bool{"is_visible": false}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/api/portal/widgets/"+widget.ID+"/visibility", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, false, data["is_visible"])
}

// TestHealthCheck 测试系统健康检查
func TestHealthCheck(t *testing.T) {
	r, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/portal/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "healthy", data["status"])
}

// TestGetDashboardStats 测试获取仪表盘统计
func TestGetDashboardStats(t *testing.T) {
	r, pm := setupTestRouter()

	// 创建一个用户仪表盘
	pm.CreateDashboard(&DashboardRequest{
		Name:   "测试仪表盘",
		Layout: LayoutGrid,
	}, "user1")

	req := httptest.NewRequest(http.MethodGet, "/api/portal/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data, ok := resp.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(4), data["total_dashboards"]) // 3模板 + 1用户
	assert.Equal(t, float64(3), data["template_count"])
	assert.Equal(t, float64(1), data["user_count"])
}
