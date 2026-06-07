// Package netdataex 提供 Netdata 高级系统监控功能
package netdataex

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)

	config := &NetdataConfig{
		NetdataURL:       "http://localhost:19999",
		RetentionDays:    30,
		SamplingInterval: 10,
		ExportEnabled:    true,
	}

	manager := NewManager(config)

	// 添加测试指标数据
	now := time.Now()
	manager.AddMetricPoint("cpu_usage", MetricPoint{
		Timestamp: now,
		Value:     25.5,
		Labels:    map[string]string{"host": "nas"},
	}, &MetricSeries{
		Name: "cpu_usage",
		Unit: "%",
		Type: MetricTypeGauge,
	})

	manager.AddMetricPoint("memory_usage", MetricPoint{
		Timestamp: now,
		Value:     60.0,
		Labels:    map[string]string{"host": "nas"},
	}, &MetricSeries{
		Name: "memory_usage",
		Unit: "%",
		Type: MetricTypeGauge,
	})

	// 添加测试告警事件
	manager.AddAlertEvent(AlertEvent{
		ID:        "alert-1",
		RuleID:    "rule-1",
		Metric:    "cpu_usage",
		Value:     90.0,
		Threshold: 80.0,
		Severity:  SeverityWarning,
		Message:   "CPU usage high",
		CreatedAt: now,
	})

	// 添加测试仪表板
	manager.CreateDashboard(Dashboard{
		ID:   "dash-1",
		Name: "System Overview",
		Panels: []Panel{
			{
				ID:       "panel-1",
				Title:    "CPU Usage",
				Type:     PanelTypeLine,
				Metrics:  []string{"cpu_usage"},
				Width:    6,
				Height:   4,
				Position: Position{X: 0, Y: 0},
			},
		},
		RefreshInterval: 10,
		IsDefault:       true,
	})

	router := gin.New()
	handlers := NewHandlers(manager)
	handlers.RegisterRoutes(router.Group("/api/v1"))

	return router, manager
}

func TestGetMetrics(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("获取存在的指标", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/metrics/cpu_usage", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		assert.Equal(t, "cpu_usage", data["name"])
	})

	t.Run("获取不存在的指标", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/metrics/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGetLatestMetric(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("获取最新指标", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/metrics/cpu_usage/latest", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		assert.Equal(t, 25.5, data["value"])
	})
}

func TestGetAllMetrics(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("获取所有指标", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/metrics", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].([]interface{})
		assert.GreaterOrEqual(t, len(data), 2)
	})
}

func TestCreateAlertRule(t *testing.T) {
	router, manager := setupTestRouter()

	t.Run("创建告警规则", func(t *testing.T) {
		rule := AlertRule{
			ID:        "rule-test",
			Name:      "High CPU",
			Metric:    "cpu_usage",
			Condition: ConditionGT,
			Threshold: 80.0,
			Severity:  SeverityWarning,
			Enabled:   true,
		}

		body, _ := json.Marshal(rule)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/monitoring/alerts/rules", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		rules, _ := manager.GetAlertRules()
		assert.Len(t, rules, 1)
		assert.Equal(t, "rule-test", rules[0].ID)
	})

	t.Run("无效请求体", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/monitoring/alerts/rules", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGetAlertRules(t *testing.T) {
	router, manager := setupTestRouter()

	// 先创建一个规则
	rule := AlertRule{
		ID:        "rule-1",
		Name:      "High CPU",
		Metric:    "cpu_usage",
		Condition: ConditionGT,
		Threshold: 80.0,
		Severity:  SeverityWarning,
		Enabled:   true,
	}
	manager.CreateAlertRule(rule)

	t.Run("获取所有告警规则", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/alerts/rules", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].([]interface{})
		assert.Len(t, data, 1)
	})
}

func TestGetAlertEvents(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("获取所有告警事件", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/alerts/events", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].([]interface{})
		assert.Len(t, data, 1)
	})

	t.Run("按级别过滤告警事件", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/alerts/events?severity=warning", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].([]interface{})
		assert.Len(t, data, 1)
	})

	t.Run("限制返回数量", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/alerts/events?limit=1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].([]interface{})
		assert.Len(t, data, 1)
	})
}

func TestAcknowledgeAlert(t *testing.T) {
	router, manager := setupTestRouter()

	t.Run("确认告警", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"user": "admin"})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/monitoring/alerts/events/alert-1/ack", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		events, _ := manager.GetAlertEvents("", 10)
		for _, e := range events {
			if e.ID == "alert-1" {
				assert.True(t, e.Acknowledged)
				assert.Equal(t, "admin", e.AckedBy)
			}
		}
	})

	t.Run("确认不存在的告警", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"user": "admin"})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/monitoring/alerts/events/nonexistent/ack", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestCreateDashboard(t *testing.T) {
	router, manager := setupTestRouter()

	t.Run("创建仪表板", func(t *testing.T) {
		dashboard := Dashboard{
			ID:   "dash-test",
			Name: "Test Dashboard",
			Panels: []Panel{
				{
					ID:       "panel-1",
					Title:    "Test Panel",
					Type:     PanelTypeGauge,
					Metrics:  []string{"cpu_usage"},
					Width:    4,
					Height:   3,
					Position: Position{X: 0, Y: 0},
				},
			},
			RefreshInterval: 5,
		}

		body, _ := json.Marshal(dashboard)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/monitoring/dashboards", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		d, _ := manager.GetDashboard("dash-test")
		assert.Equal(t, "Test Dashboard", d.Name)
	})

	t.Run("无效请求体", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/monitoring/dashboards", bytes.NewBufferString("invalid"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGetDashboard(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("获取存在的仪表板", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/dashboards/dash-1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		assert.Equal(t, "System Overview", data["name"])
	})

	t.Run("获取不存在的仪表板", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/dashboards/nonexistent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestListDashboards(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("获取所有仪表板", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/dashboards", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].([]interface{})
		assert.Len(t, data, 1)
	})
}

func TestUpdateDashboard(t *testing.T) {
	router, manager := setupTestRouter()

	t.Run("更新存在的仪表板", func(t *testing.T) {
		dashboard := Dashboard{
			Name: "Updated Dashboard",
			Panels: []Panel{
				{
					ID:       "panel-new",
					Title:    "New Panel",
					Type:     PanelTypeBar,
					Metrics:  []string{"memory_usage"},
					Width:    6,
					Height:   4,
					Position: Position{X: 0, Y: 0},
				},
			},
			RefreshInterval: 15,
		}

		body, _ := json.Marshal(dashboard)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/monitoring/dashboards/dash-1", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		d, _ := manager.GetDashboard("dash-1")
		assert.Equal(t, "Updated Dashboard", d.Name)
	})

	t.Run("更新不存在的仪表板", func(t *testing.T) {
		dashboard := Dashboard{Name: "Test"}
		body, _ := json.Marshal(dashboard)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/monitoring/dashboards/nonexistent", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGetHealthReport(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("获取健康报告", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		assert.Equal(t, float64(95), data["score"])
	})
}

func TestExportMetrics(t *testing.T) {
	router, _ := setupTestRouter()

	t.Run("导出 JSON 格式", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/export?format=json", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})

	t.Run("导出 Prometheus 格式", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/export?format=prometheus", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	})

	t.Run("不支持的格式", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/monitoring/export?format=csv", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
