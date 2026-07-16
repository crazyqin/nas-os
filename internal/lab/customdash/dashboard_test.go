package customdash

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter() (*gin.Engine, *DashboardManager) {
	dm := NewDashboardManager()
	handler := NewHandler(dm)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)
	return router, dm
}

func TestDefaultDashboards(t *testing.T) {
	dm := NewDashboardManager()

	// 验证3个默认仪表盘
	dashboards := dm.ListDashboards()
	assert.Equal(t, 3, len(dashboards), "should have 3 default dashboards")

	// 验证默认仪表盘名称
	names := make(map[string]bool)
	for _, d := range dashboards {
		names[d.Name] = true
	}
	assert.True(t, names["系统概览"])
	assert.True(t, names["存储监控"])
	assert.True(t, names["网络监控"])

	// 验证默认widget数量
	assert.Equal(t, 9, dm.WidgetCount(), "should have 9 default widgets total")
}

func TestDashboardCRUD(t *testing.T) {
	dm := NewDashboardManager()

	// 创建
	dash, err := dm.CreateDashboard("自定义仪表盘", "测试用")
	assert.NoError(t, err)
	assert.Equal(t, "自定义仪表盘", dash.Name)
	assert.Equal(t, 4, dm.DashboardCount())

	// 添加widget
	w, err := dm.AddWidget(dash.ID, &Widget{
		Type:     WidgetCPU,
		Title:    "CPU测试",
		Position: Position{X: 0, Y: 0},
		Size:     Size{W: 6, H: 4},
	})
	assert.NoError(t, err)
	assert.Equal(t, WidgetCPU, w.Type)

	// 查询
	widgets, err := dm.GetWidgets(dash.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(widgets))

	// 更新widget
	updated, err := dm.UpdateWidget(dash.ID, w.ID, &Widget{
		Title: "CPU更新",
	})
	assert.NoError(t, err)
	assert.Equal(t, "CPU更新", updated.Title)

	// 删除widget
	err = dm.DeleteWidget(dash.ID, w.ID)
	assert.NoError(t, err)
	widgets, _ = dm.GetWidgets(dash.ID)
	assert.Equal(t, 0, len(widgets))

	// 删除仪表盘
	err = dm.DeleteDashboard(dash.ID)
	assert.NoError(t, err)
	assert.Equal(t, 3, dm.DashboardCount())
}

func TestExportImport(t *testing.T) {
	dm := NewDashboardManager()

	// 导出系统概览
	data, err := dm.ExportDashboard("default-system")
	assert.NoError(t, err)
	assert.Equal(t, "系统概览", data.Dashboard.Name)
	assert.Equal(t, 5, len(data.Widgets))

	// 导入（应生成新ID）
	dash, err := dm.ImportDashboard(data)
	assert.NoError(t, err)
	assert.NotEqual(t, "default-system", dash.ID)
	assert.Equal(t, 4, dm.DashboardCount())

	// 导出为JSON再导入
	raw, err := dm.ExportDashboardJSON("default-system")
	assert.NoError(t, err)
	dash2, err := dm.ImportDashboardJSON(raw)
	assert.NoError(t, err)
	assert.Equal(t, 5, dm.DashboardCount())
	assert.Equal(t, "系统概览", dash2.Name)
}

func TestDataCollection(t *testing.T) {
	dm := NewDashboardManager()

	// 获取一个widget
	widgets, _ := dm.GetWidgets("default-system")
	cpuWidget := widgets[0]

	// 手动采集数据
	dm.collectSample(cpuWidget)

	// 验证数据已记录
	data, err := dm.GetWidgetData("default-system", cpuWidget.ID)
	assert.NoError(t, err)
	assert.NotNil(t, data.Latest)
	assert.True(t, len(data.Samples) > 0)
	assert.Equal(t, 1, dm.WidgetDataSize(cpuWidget.ID))
}

func TestThresholdAlert(t *testing.T) {
	dm := NewDashboardManager()

	// 网络流量widget已配置阈值告警
	widgets, _ := dm.GetWidgets("default-network")
	assert.Equal(t, 1, len(widgets))
	netWidget := widgets[0]
	assert.True(t, netWidget.Threshold.Enabled)
	assert.Equal(t, "rx_mbps", netWidget.Threshold.Metric)
	assert.Equal(t, OpGT, netWidget.Threshold.Operator)
	assert.Equal(t, 900.0, netWidget.Threshold.Value)

	// 验证阈值逻辑
	sample := &DataSample{
		Timestamp: time.Now(),
		Value:     450.0,
		Extra:     map[string]float64{"rx_mbps": 950.0, "tx_mbps": 150.0},
	}
	// 这里只验证不panic，日志中会输出告警
	dm.checkThreshold(netWidget, sample)
}

func TestHTTPListDashboards(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/list", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(3), resp["total"])
}

func TestHTTPCreateDelete(t *testing.T) {
	router, _ := setupTestRouter()

	// 创建
	body, _ := json.Marshal(map[string]string{
		"name":        "HTTP测试",
		"description": "通过API创建",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var dash Dashboard
	json.Unmarshal(w.Body.Bytes(), &dash)
	assert.Equal(t, "HTTP测试", dash.Name)

	// 删除
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/dashboard/"+dash.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPWidgetOperations(t *testing.T) {
	router, dm := setupTestRouter()

	// 创建测试仪表盘
	dash, _ := dm.CreateDashboard("widget测试", "")

	// 添加widget
	body, _ := json.Marshal(map[string]interface{}{
		"type":     "cpu",
		"title":    "API-CPU",
		"position": map[string]int{"x": 0, "y": 0},
		"size":     map[string]int{"w": 6, "h": 4},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/"+dash.ID+"/widgets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var widget Widget
	json.Unmarshal(w.Body.Bytes(), &widget)
	assert.Equal(t, "API-CPU", widget.Title)

	// 获取widget列表
	req = httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/"+dash.ID+"/widgets", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var listResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	assert.Equal(t, float64(1), listResp["total"])

	// 删除widget
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/dashboard/"+dash.ID+"/widgets/"+widget.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPExportImport(t *testing.T) {
	router, _ := setupTestRouter()

	// 导出
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/default-system/export", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var exportData ExportData
	json.Unmarshal(w.Body.Bytes(), &exportData)
	assert.Equal(t, "系统概览", exportData.Dashboard.Name)

	// 导入
	body, _ := json.Marshal(exportData)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestStartStopSampler(t *testing.T) {
	dm := NewDashboardManager()

	dm.StartSampler(50 * time.Millisecond)
	assert.True(t, dm.running)

	time.Sleep(200 * time.Millisecond)
	dm.StopSampler()
	assert.False(t, dm.running)
}

func TestInvalidOperations(t *testing.T) {
	dm := NewDashboardManager()

	// 获取不存在的仪表盘
	_, err := dm.GetDashboard("nonexistent")
	assert.Error(t, err)

	// 删除不存在的仪表盘
	err = dm.DeleteDashboard("nonexistent")
	assert.Error(t, err)

	// 在不存在的仪表盘添加widget
	_, err = dm.AddWidget("nonexistent", &Widget{Type: WidgetCPU})
	assert.Error(t, err)

	// 导出不存在的仪表盘
	_, err = dm.ExportDashboard("nonexistent")
	assert.Error(t, err)

	// 导入无效数据
	_, err = dm.ImportDashboard(nil)
	assert.Error(t, err)
}
