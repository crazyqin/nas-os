// Package smarthealthpredict 测试
package smarthealthpredict

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	m, err := NewManager(t.TempDir(), WithLogger(logger))
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	return m
}

func setupTestRouter(t *testing.T, manager *Manager) *gin.Engine {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	handler := NewHandler(manager, logger)
	r := gin.New()
	api := r.Group("/api/v1")
	handler.RegisterRoutes(api)
	return r
}

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	t.Run("创建成功", func(t *testing.T) {
		manager, err := NewManager(t.TempDir())
		if err != nil {
			t.Fatalf("创建管理器失败: %v", err)
		}
		if manager == nil {
			t.Fatal("管理器为 nil")
		}
	})

	t.Run("路径为空", func(t *testing.T) {
		_, err := NewManager("")
		if err != ErrPathRequired {
			t.Fatalf("期望 ErrPathRequired，得到 %v", err)
		}
	})

	t.Run("自定义选项", func(t *testing.T) {
		logger, _ := zap.NewDevelopment()
		manager, err := NewManager(t.TempDir(),
			WithModel(ModelMLBased),
			WithRetentionDays(30),
			WithPredictInterval(12*time.Hour),
			WithLogger(logger),
		)
		if err != nil {
			t.Fatalf("创建管理器失败: %v", err)
		}
		if manager.model != ModelMLBased {
			t.Errorf("模型不匹配: got %v, want %v", manager.model, ModelMLBased)
		}
		if manager.retentionDays != 30 {
			t.Errorf("保留天数不匹配: got %d, want 30", manager.retentionDays)
		}
		if manager.logger == nil {
			t.Error("日志器不应为 nil")
		}
	})
}

func TestScanDisk(t *testing.T) {
	manager := setupTestManager(t)

	report, err := manager.ScanDisk(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("扫描磁盘失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告为 nil")
	}

	if report.Score < 0 || report.Score > 100 {
		t.Errorf("健康评分超出范围: %d", report.Score)
	}

	if report.Status == "" {
		t.Error("健康状态为空")
	}

	if report.Timestamp.IsZero() {
		t.Error("时间戳为零值")
	}

	if len(report.Attributes) == 0 {
		t.Error("S.M.A.R.T 属性不应为空")
	}

	if report.Disk.Device != "/dev/sda" {
		t.Errorf("设备路径不匹配: got %s", report.Disk.Device)
	}
}

func TestHealthStatus(t *testing.T) {
	manager := setupTestManager(t)

	tests := []struct {
		score    int
		expected HealthStatus
	}{
		{95, StatusExcellent},
		{80, StatusGood},
		{60, StatusFair},
		{40, StatusPoor},
		{20, StatusCritical},
	}

	for _, tt := range tests {
		status := manager.getHealthStatus(tt.score)
		if status != tt.expected {
			t.Errorf("score=%d: got %v, want %v", tt.score, status, tt.expected)
		}
	}
}

func TestAlertRules(t *testing.T) {
	manager := setupTestManager(t)

	// 测试温度告警
	info := DiskInfo{
		Device:      "/dev/sda",
		Temperature: 65,
		PowerOn:     1000,
		SMARTPassed: true,
	}

	attrs := []SMARTAttribute{
		{Name: "Temperature_Celsius", RawValue: 65},
	}

	alerts := manager.checkAlerts(info, attrs, 80)
	if len(alerts) == 0 {
		t.Error("期望有温度告警，但没有")
	}

	// 测试健康分告警
	info.Temperature = 30
	alerts = manager.checkAlerts(info, attrs, 25)
	foundHealthAlert := false
	for _, a := range alerts {
		if a.Type == "health" {
			foundHealthAlert = true
			break
		}
	}
	if !foundHealthAlert {
		t.Error("期望有健康分告警，但没有")
	}
}

func TestTrendAnalysis(t *testing.T) {
	manager := setupTestManager(t)

	device := "/dev/sda"

	// 添加历史数据
	for i := 0; i < 10; i++ {
		manager.history[device] = append(manager.history[device], HealthSnapshot{
			Timestamp:   time.Now().AddDate(0, 0, -10+i),
			Score:       90 - i*2,
			Temperature: 40 + i,
			PowerOn:     8000 + int64(i*100),
		})
	}

	trend := manager.analyzeTrend(device)
	if trend == nil {
		t.Fatal("趋势分析为 nil")
	}

	if trend.HealthTrend != "degrading" {
		t.Errorf("期望 degrading，得到 %v", trend.HealthTrend)
	}
}

func TestPredictions(t *testing.T) {
	manager := setupTestManager(t)

	info := DiskInfo{
		Device: "/dev/sda",
		Type:   DiskTypeHDD,
		Health: 80,
	}

	attrs := []SMARTAttribute{
		{Name: "Reallocated_Sector_Ct", RawValue: 150, Critical: true},
	}

	trend := &TrendAnalysis{
		HealthTrend: "stable",
		DeclineRate: 0,
	}

	predictions := manager.predictFailures("/dev/sda", info, attrs, trend)
	if len(predictions) == 0 {
		t.Error("期望有故障预测，但没有")
	}
}

func TestGetDiskList(t *testing.T) {
	manager := setupTestManager(t)

	// 扫描两个磁盘
	manager.ScanDisk(context.Background(), "/dev/sda")
	manager.ScanDisk(context.Background(), "/dev/sdb")

	disks := manager.GetDiskList()
	if len(disks) != 2 {
		t.Errorf("期望 2 个磁盘，得到 %d", len(disks))
	}
}

func TestGetDiskHistory(t *testing.T) {
	manager := setupTestManager(t)

	device := "/dev/sda"

	// 扫描磁盘生成历史
	manager.ScanDisk(context.Background(), device)

	history := manager.GetDiskHistory(device, 30)
	if len(history) == 0 {
		t.Error("期望有历史数据，但没有")
	}
}

func TestGetAlerts(t *testing.T) {
	manager := setupTestManager(t)

	// 扫描磁盘
	manager.ScanDisk(context.Background(), "/dev/sda")

	alerts := manager.GetAlerts()
	// 告警数量取决于模拟数据
	t.Logf("获取到 %d 个告警", len(alerts))
}

func TestGetSystemStatus(t *testing.T) {
	manager := setupTestManager(t)

	// 扫描几个磁盘
	manager.ScanDisk(context.Background(), "/dev/sda")
	manager.ScanDisk(context.Background(), "/dev/sdb")

	status := manager.GetSystemStatus()
	totalDisks, ok := status["totalDisks"].(int)
	if !ok || totalDisks != 2 {
		t.Errorf("总磁盘数应为 2，得到 %v", status["totalDisks"])
	}

	avgScore, ok := status["averageScore"].(int)
	if !ok || avgScore < 0 || avgScore > 100 {
		t.Errorf("平均分数应为 0-100，得到 %v", status["averageScore"])
	}

	statusCount, ok := status["statusCount"].(map[string]int)
	if !ok {
		t.Fatal("statusCount 类型不正确")
	}
	if _, exists := statusCount["excellent"]; !exists {
		t.Error("statusCount 应包含 excellent 键")
	}
}

// ========== Handler/Router 测试 ==========

func TestHandlerScanDisk(t *testing.T) {
	manager := setupTestManager(t)
	r := setupTestRouter(t, manager)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/health/scan?device=/dev/sda", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}

	var resp response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("响应码应为 0，得到 %d", resp.Code)
	}
}

func TestHandlerScanDiskMissingDevice(t *testing.T) {
	manager := setupTestManager(t)
	r := setupTestRouter(t, manager)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/storage/health/scan", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("缺少设备参数应返回 400，得到 %d", w.Code)
	}
}

func TestHandlerListDisks(t *testing.T) {
	manager := setupTestManager(t)
	r := setupTestRouter(t, manager)

	// 先扫描一个磁盘
	manager.ScanDisk(context.Background(), "/dev/sda")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/health/disks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetDiskReport(t *testing.T) {
	manager := setupTestManager(t)
	r := setupTestRouter(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/health/report?device=/dev/sda", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Logf("响应: %s", w.Body.String())
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetDiskHistory(t *testing.T) {
	manager := setupTestManager(t)
	r := setupTestRouter(t, manager)

	// 先扫描生成历史
	manager.ScanDisk(context.Background(), "/dev/sda")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/health/history?device=/dev/sda&days=30", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetDiskHistoryMissingDevice(t *testing.T) {
	manager := setupTestManager(t)
	r := setupTestRouter(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/health/history", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("缺少设备参数应返回 400，得到 %d", w.Code)
	}
}

func TestHandlerGetAlerts(t *testing.T) {
	manager := setupTestManager(t)
	r := setupTestRouter(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/health/alerts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}

func TestHandlerGetStatus(t *testing.T) {
	manager := setupTestManager(t)
	r := setupTestRouter(t, manager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storage/health/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为 200，得到 %d", w.Code)
	}
}
