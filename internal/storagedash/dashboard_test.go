package storagedash

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestLogger 创建测试用 logger
func newTestLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

// --- 类型定义测试 ---

func TestTypesSerialization(t *testing.T) {
	// 验证 StorageOverview JSON 序列化
	overview := StorageOverview{
		TotalCapacity: 1024 * 1024 * 1024 * 100, // 100 GB
		UsedCapacity:  1024 * 1024 * 1024 * 60,  // 60 GB
		FreeCapacity:  1024 * 1024 * 1024 * 40,  // 40 GB
		Utilization:   0.6,
		Health:        "healthy",
		Pools: []PoolSummary{
			{Name: "tank", Status: "online", UsedBytes: 60 * 1024 * 1024 * 1024, TotalBytes: 100 * 1024 * 1024 * 1024, DiskCount: 4, RAIDLevel: "raidz1"},
		},
		Tiers: []TierSummary{
			{Tier: "hot", UsedBytes: 10 * 1024 * 1024 * 1024, TotalBytes: 20 * 1024 * 1024 * 1024, FileCount: 1500, MigrationPending: 5},
		},
	}

	data, err := json.Marshal(overview)
	if err != nil {
		t.Fatalf("序列化 StorageOverview 失败: %v", err)
	}

	var decoded StorageOverview
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化 StorageOverview 失败: %v", err)
	}

	if decoded.TotalCapacity != overview.TotalCapacity {
		t.Errorf("TotalCapacity 不匹配: got %d, want %d", decoded.TotalCapacity, overview.TotalCapacity)
	}
	if decoded.Health != "healthy" {
		t.Errorf("Health 不匹配: got %s, want healthy", decoded.Health)
	}
	if len(decoded.Pools) != 1 {
		t.Errorf("Pools 长度不匹配: got %d, want 1", len(decoded.Pools))
	}
	if len(decoded.Tiers) != 1 {
		t.Errorf("Tiers 长度不匹配: got %d, want 1", len(decoded.Tiers))
	}
}

func TestAlertSerialization(t *testing.T) {
	summary := AlertSummary{
		Critical: 1,
		Warning:  2,
		Info:     3,
		RecentAlerts: []Alert{
			{Level: "critical", Message: "池故障", Source: "tank"},
			{Level: "warning", Message: "容量高", Source: "tank"},
		},
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("序列化 AlertSummary 失败: %v", err)
	}

	var decoded AlertSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("反序列化 AlertSummary 失败: %v", err)
	}

	if decoded.Critical != 1 || decoded.Warning != 2 || decoded.Info != 3 {
		t.Errorf("告警计数不匹配: got critical=%d warning=%d info=%d", decoded.Critical, decoded.Warning, decoded.Info)
	}
}

// --- Dashboard 引擎测试 ---

func TestNewDashboard(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	if d == nil {
		t.Fatal("NewDashboard 返回 nil")
	}
	if d.logger == nil {
		t.Error("logger 未初始化")
	}
	if len(d.poolProviders) != 0 {
		t.Errorf("初始 poolProviders 应为空，got %d", len(d.poolProviders))
	}
	if len(d.tierProviders) != 0 {
		t.Errorf("初始 tierProviders 应为空，got %d", len(d.tierProviders))
	}
}

func TestRegisterPoolProvider(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{{Name: "tank", Status: "online"}}, nil
	})
	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{{Name: "backup", Status: "online"}}, nil
	})

	if len(d.poolProviders) != 2 {
		t.Errorf("poolProviders 数量不正确: got %d, want 2", len(d.poolProviders))
	}
}

func TestRegisterTierProvider(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterTierProvider(func() ([]TierSummary, error) {
		return []TierSummary{{Tier: "hot"}}, nil
	})

	if len(d.tierProviders) != 1 {
		t.Errorf("tierProviders 数量不正确: got %d, want 1", len(d.tierProviders))
	}
}

func TestGetOverviewNoProviders(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	overview, err := d.GetOverview()
	if err != nil {
		t.Fatalf("GetOverview 失败: %v", err)
	}
	if overview.TotalCapacity != 0 {
		t.Errorf("无数据源时 TotalCapacity 应为 0，got %d", overview.TotalCapacity)
	}
	if overview.Health != "healthy" {
		t.Errorf("无数据源时 Health 应为 healthy，got %s", overview.Health)
	}
	if len(overview.Pools) != 0 {
		t.Errorf("无数据源时 Pools 应为空，got %d", len(overview.Pools))
	}
}

func TestGetOverviewWithProviders(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{
			{
				Name:       "tank",
				Status:     "online",
				UsedBytes:  60 * 1024 * 1024 * 1024,
				TotalBytes: 100 * 1024 * 1024 * 1024,
				DiskCount:  4,
				RAIDLevel:  "raidz1",
			},
		}, nil
	})

	d.RegisterTierProvider(func() ([]TierSummary, error) {
		return []TierSummary{
			{Tier: "hot", UsedBytes: 10 * 1024 * 1024 * 1024, TotalBytes: 20 * 1024 * 1024 * 1024, FileCount: 500},
			{Tier: "cold", UsedBytes: 50 * 1024 * 1024 * 1024, TotalBytes: 80 * 1024 * 1024 * 1024, FileCount: 10000},
		}, nil
	})

	overview, err := d.GetOverview()
	if err != nil {
		t.Fatalf("GetOverview 失败: %v", err)
	}

	if overview.TotalCapacity != 100*1024*1024*1024 {
		t.Errorf("TotalCapacity 不正确: got %d", overview.TotalCapacity)
	}
	if overview.UsedCapacity != 60*1024*1024*1024 {
		t.Errorf("UsedCapacity 不正确: got %d", overview.UsedCapacity)
	}
	if overview.FreeCapacity != 40*1024*1024*1024 {
		t.Errorf("FreeCapacity 不正确: got %d", overview.FreeCapacity)
	}
	if overview.Utilization != 0.6 {
		t.Errorf("Utilization 不正确: got %f, want 0.6", overview.Utilization)
	}
	if overview.Health != "healthy" {
		t.Errorf("Health 不正确: got %s, want healthy", overview.Health)
	}
	if len(overview.Pools) != 1 {
		t.Errorf("Pools 长度不正确: got %d, want 1", len(overview.Pools))
	}
	if len(overview.Tiers) != 2 {
		t.Errorf("Tiers 长度不正确: got %d, want 2", len(overview.Tiers))
	}
}

func TestGetOverviewProviderError(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return nil, errors.New("连接失败")
	})
	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{{Name: "tank", Status: "online", TotalBytes: 1000}}, nil
	})

	// 即使一个 provider 失败，另一个仍应返回结果
	overview, err := d.GetOverview()
	if err != nil {
		t.Fatalf("GetOverview 不应因单个 provider 失败而返回错误: %v", err)
	}
	if len(overview.Pools) != 1 {
		t.Errorf("应有 1 个池，got %d", len(overview.Pools))
	}
}

func TestGetOverviewAllProvidersFail(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return nil, errors.New("失败1")
	})
	d.RegisterTierProvider(func() ([]TierSummary, error) {
		return nil, errors.New("失败2")
	})

	overview, err := d.GetOverview()
	if err != nil {
		t.Fatalf("所有 provider 失败时 GetOverview 不应返回错误: %v", err)
	}
	if overview.TotalCapacity != 0 {
		t.Errorf("所有 provider 失败时 TotalCapacity 应为 0，got %d", overview.TotalCapacity)
	}
}

// --- 缓存测试 ---

func TestCaching(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	callCount := 0
	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		callCount++
		return []PoolSummary{{Name: "tank", Status: "online", TotalBytes: 1000}}, nil
	})

	// 第一次调用
	_, err := d.GetOverview()
	if err != nil {
		t.Fatalf("第一次 GetOverview 失败: %v", err)
	}
	if callCount != 1 {
		t.Errorf("第一次调用后 provider 应被调用 1 次，got %d", callCount)
	}

	// 第二次调用（应使用缓存）
	_, err = d.GetOverview()
	if err != nil {
		t.Fatalf("第二次 GetOverview 失败: %v", err)
	}
	if callCount != 1 {
		t.Errorf("缓存命中时 provider 不应被重复调用，got %d", callCount)
	}
}

func TestRefreshCache(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	callCount := 0
	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		callCount++
		return []PoolSummary{{Name: "tank", Status: "online", TotalBytes: 1000}}, nil
	})

	// 初始调用
	d.GetOverview()
	if callCount != 1 {
		t.Fatalf("初始调用应为 1 次，got %d", callCount)
	}

	// 刷新缓存
	err := d.RefreshCache()
	if err != nil {
		t.Fatalf("RefreshCache 失败: %v", err)
	}
	if callCount != 2 {
		t.Errorf("RefreshCache 后 provider 应被调用 2 次，got %d", callCount)
	}
}

func TestRefreshCacheAllFail(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return nil, errors.New("全部失败")
	})

	err := d.RefreshCache()
	if err == nil {
		t.Error("所有 provider 失败时 RefreshCache 应返回错误")
	}
}

// --- 健康评估测试 ---

func TestEvaluateHealth_FaultedPool(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	pools := []PoolSummary{
		{Name: "tank", Status: "faulted"},
	}
	health := d.evaluateHealth(pools, 0.5)
	if health != "critical" {
		t.Errorf("faulted 池应为 critical，got %s", health)
	}
}

func TestEvaluateHealth_OfflinePool(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	pools := []PoolSummary{
		{Name: "tank", Status: "offline"},
	}
	health := d.evaluateHealth(pools, 0.5)
	if health != "critical" {
		t.Errorf("offline 池应为 critical，got %s", health)
	}
}

func TestEvaluateHealth_DegradedPool(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	pools := []PoolSummary{
		{Name: "tank", Status: "degraded"},
	}
	health := d.evaluateHealth(pools, 0.5)
	if health != "warning" {
		t.Errorf("degraded 池应为 warning，got %s", health)
	}
}

func TestEvaluateHealth_HighUtilization(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	pools := []PoolSummary{
		{Name: "tank", Status: "online"},
	}
	health := d.evaluateHealth(pools, 0.95)
	if health != "warning" {
		t.Errorf("使用率 95%% 应为 warning，got %s", health)
	}
}

func TestEvaluateHealth_Normal(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	pools := []PoolSummary{
		{Name: "tank", Status: "online"},
	}
	health := d.evaluateHealth(pools, 0.5)
	if health != "healthy" {
		t.Errorf("正常状态应为 healthy，got %s", health)
	}
}

func TestEvaluateHealth_MultiplePools(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	// 一个正常，一个故障 → critical（故障优先级更高）
	pools := []PoolSummary{
		{Name: "tank", Status: "online"},
		{Name: "backup", Status: "faulted"},
	}
	health := d.evaluateHealth(pools, 0.3)
	if health != "critical" {
		t.Errorf("混合状态应为 critical，got %s", health)
	}
}

// --- 趋势数据测试 ---

func TestGetCapacityTrends(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{
			{
				Name:       "tank",
				Status:     "online",
				UsedBytes:  50 * 1024 * 1024 * 1024,
				TotalBytes: 100 * 1024 * 1024 * 1024,
			},
		}, nil
	})

	trends, err := d.GetCapacityTrends(7)
	if err != nil {
		t.Fatalf("GetCapacityTrends 失败: %v", err)
	}
	if len(trends) != 7 {
		t.Errorf("趋势数据长度不正确: got %d, want 7", len(trends))
	}

	// 验证每天都有数据
	for i, trend := range trends {
		if trend.UsedBytes < 0 {
			t.Errorf("第 %d 天 UsedBytes 不应为负: %d", i, trend.UsedBytes)
		}
	}
}

func TestGetCapacityTrends_DefaultDays(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	// days=0 应默认为 7
	trends, err := d.GetCapacityTrends(0)
	if err != nil {
		t.Fatalf("GetCapacityTrends(0) 失败: %v", err)
	}
	if len(trends) != 7 {
		t.Errorf("默认天数应为 7，got %d", len(trends))
	}
}

func TestGetCapacityTrends_NegativeDays(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	// days=-5 应默认为 7
	trends, err := d.GetCapacityTrends(-5)
	if err != nil {
		t.Fatalf("GetCapacityTrends(-5) 失败: %v", err)
	}
	if len(trends) != 7 {
		t.Errorf("负天数应默认为 7，got %d", len(trends))
	}
}

func TestTrimTrends(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	trends := []CapacityTrend{
		{UsedBytes: 100},
		{UsedBytes: 200},
		{UsedBytes: 300},
		{UsedBytes: 400},
		{UsedBytes: 500},
	}

	trimmed := d.trimTrends(trends, 3)
	if len(trimmed) != 3 {
		t.Errorf("trimTrends 结果长度不正确: got %d, want 3", len(trimmed))
	}
	// 应取最后 3 条
	if trimmed[0].UsedBytes != 300 {
		t.Errorf("trimTrends 应从第 3 条开始，got UsedBytes=%d", trimmed[0].UsedBytes)
	}

	// 请求天数大于实际数据
	trimmed = d.trimTrends(trends, 10)
	if len(trimmed) != 5 {
		t.Errorf("trimTrends 超出范围时应返回全部，got %d", len(trimmed))
	}
}

// --- 告警测试 ---

func TestGetAlerts(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{
			{Name: "tank", Status: "faulted", UsedBytes: 90, TotalBytes: 100},
			{Name: "backup", Status: "degraded", UsedBytes: 50, TotalBytes: 100},
			{Name: "archive", Status: "online", UsedBytes: 30, TotalBytes: 100},
		}, nil
	})

	alerts, err := d.GetAlerts()
	if err != nil {
		t.Fatalf("GetAlerts 失败: %v", err)
	}

	// tank: faulted → 1 critical, used 90% → 1 warning (池级)
	// backup: degraded → 1 warning
	// archive: online, 30% → 无额外告警
	// 预期: critical >= 1, warning >= 2
	if alerts.Critical < 1 {
		t.Errorf("应有至少 1 个 critical 告警，got %d", alerts.Critical)
	}
	if alerts.Warning < 2 {
		t.Errorf("应有至少 2 个 warning 告警，got %d", alerts.Warning)
	}
}

func TestGetAlerts_NoAlerts(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{
			{Name: "tank", Status: "online", UsedBytes: 30, TotalBytes: 100},
		}, nil
	})

	alerts, err := d.GetAlerts()
	if err != nil {
		t.Fatalf("GetAlerts 失败: %v", err)
	}

	if alerts.Critical != 0 || alerts.Warning != 0 || alerts.Info != 0 {
		t.Errorf("正常状态不应有告警: critical=%d warning=%d info=%d",
			alerts.Critical, alerts.Warning, alerts.Info)
	}
}

func TestGetAlerts_MigrationPending(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{{Name: "tank", Status: "online", UsedBytes: 30, TotalBytes: 100}}, nil
	})
	d.RegisterTierProvider(func() ([]TierSummary, error) {
		return []TierSummary{
			{Tier: "hot", MigrationPending: 150}, // 超过阈值 100
		}, nil
	})

	alerts, err := d.GetAlerts()
	if err != nil {
		t.Fatalf("GetAlerts 失败: %v", err)
	}

	if alerts.Warning < 1 {
		t.Errorf("待迁移 150 应触发 warning，got %d", alerts.Warning)
	}
}

func TestGetAlerts_CapacityThresholds(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{
			// 使用率 97% → critical
			{Name: "almost-full", Status: "online", UsedBytes: 97, TotalBytes: 100},
			// 使用率 88% → warning
			{Name: "getting-full", Status: "online", UsedBytes: 88, TotalBytes: 100},
		}, nil
	})

	alerts, err := d.GetAlerts()
	if err != nil {
		t.Fatalf("GetAlerts 失败: %v", err)
	}

	if alerts.Critical < 1 {
		t.Errorf("97%% 使用率应触发 critical，got %d", alerts.Critical)
	}
	if alerts.Warning < 1 {
		t.Errorf("88%% 使用率应触发 warning，got %d", alerts.Warning)
	}
}

func TestGetAlerts_RecentAlertsLimit(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	// 创建 60 个故障池以测试告警上限
	pools := make([]PoolSummary, 60)
	for i := range pools {
		pools[i] = PoolSummary{
			Name:       fmt.Sprintf("pool-%d", i),
			Status:     "faulted",
			UsedBytes:  99,
			TotalBytes: 100,
		}
	}

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return pools, nil
	})

	alerts, err := d.GetAlerts()
	if err != nil {
		t.Fatalf("GetAlerts 失败: %v", err)
	}

	if len(alerts.RecentAlerts) > 50 {
		t.Errorf("RecentAlerts 应限制为 50 条，got %d", len(alerts.RecentAlerts))
	}
}

// --- Handler 测试 ---

func setupTestRouter(d *Dashboard) *gin.Engine {
	logger := newTestLogger()
	handler := NewHandler(d, logger)
	r := gin.New()
	rg := r.Group("/api/v1/dashboard")
	handler.RegisterRoutes(rg)
	return r
}

func TestHandleGetStorage(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)
	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{{Name: "tank", Status: "online", TotalBytes: 1000, UsedBytes: 500}}, nil
	})

	r := setupTestRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/storage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不正确: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应 JSON 解析失败: %v", err)
	}
	if resp["code"].(float64) != 0 {
		t.Errorf("响应 code 不为 0: %v", resp["code"])
	}
}

func TestHandleGetTrends(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	r := setupTestRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/trends?days=14", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不正确: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应 JSON 解析失败: %v", err)
	}
	if resp["code"].(float64) != 0 {
		t.Errorf("响应 code 不为 0: %v", resp["code"])
	}
}

func TestHandleGetTrends_InvalidDays(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	r := setupTestRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/trends?days=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("无效 days 应返回 400，got %d", w.Code)
	}
}

func TestHandleGetTrends_NegativeDays(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	r := setupTestRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/trends?days=-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("负数 days 应返回 400，got %d", w.Code)
	}
}

func TestHandleGetAlerts(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	r := setupTestRouter(d)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/alerts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不正确: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandleRefresh(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	r := setupTestRouter(d)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码不正确: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应 JSON 解析失败: %v", err)
	}
	if resp["message"] != "缓存已刷新" {
		t.Errorf("消息不正确: got %v", resp["message"])
	}
}

func TestHandleRefresh_AllFail(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)
	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return nil, errors.New("连接超时")
	})

	r := setupTestRouter(d)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("全部失败时应返回 500，got %d", w.Code)
	}
}

// --- 路由注册测试 ---

func TestRegisterRoutes(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)
	handler := NewHandler(d, logger)

	r := gin.New()
	rg := r.Group("/api/v1/dashboard")
	handler.RegisterRoutes(rg)

	// 验证路由存在
	routes := r.Routes()
	expectedPaths := map[string]string{
		"/api/v1/dashboard/storage": "GET",
		"/api/v1/dashboard/trends":  "GET",
		"/api/v1/dashboard/alerts":  "GET",
		"/api/v1/dashboard/refresh": "POST",
	}

	for path, method := range expectedPaths {
		found := false
		for _, route := range routes {
			if route.Path == path && route.Method == method {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("路由 %s %s 未注册", method, path)
		}
	}
}

// --- 并发安全测试 ---

func TestConcurrentAccess(t *testing.T) {
	logger := newTestLogger()
	d := NewDashboard(logger)

	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{{Name: "tank", Status: "online", TotalBytes: 1000}}, nil
	})
	d.RegisterTierProvider(func() ([]TierSummary, error) {
		return []TierSummary{{Tier: "hot"}}, nil
	})

	done := make(chan bool, 20)

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = d.GetOverview()
			done <- true
		}()
	}

	// 并发刷新
	for i := 0; i < 5; i++ {
		go func() {
			_ = d.RefreshCache()
			done <- true
		}()
	}

	// 并发趋势查询
	for i := 0; i < 5; i++ {
		go func() {
			_, _ = d.GetCapacityTrends(7)
			done <- true
		}()
	}

	// 等待全部完成
	for i := 0; i < 20; i++ {
		<-done
	}
}

// --- 基准测试 ---

func BenchmarkGetOverview(b *testing.B) {
	logger, _ := zap.NewProduction()
	d := NewDashboard(logger)
	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{{Name: "tank", Status: "online", TotalBytes: 1000}}, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 禁用缓存以测试真实性能
		d.mu.Lock()
		d.cachedOverview = nil
		d.mu.Unlock()
		_, _ = d.GetOverview()
	}
}

func BenchmarkGetOverview_Cached(b *testing.B) {
	logger, _ := zap.NewProduction()
	d := NewDashboard(logger)
	d.RegisterPoolProvider(func() ([]PoolSummary, error) {
		return []PoolSummary{{Name: "tank", Status: "online", TotalBytes: 1000}}, nil
	})
	// 预热缓存
	_, _ = d.GetOverview()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.GetOverview()
	}
}
