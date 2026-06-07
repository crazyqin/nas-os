package system

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestHealthChecker 创建用于测试的健康检查器.
func newTestHealthChecker(t *testing.T) (*SystemHealthChecker, func()) {
	t.Helper()
	tmpDB := "/tmp/test_healthcheck_" + t.Name() + ".db"
	monitor, err := NewMonitor(tmpDB)
	if err != nil {
		t.Fatalf("创建监控器失败：%v", err)
	}
	cleanup := func() {
		monitor.Close()
		_ = os.Remove(tmpDB)
	}
	hc := NewSystemHealthChecker(monitor, nil)
	return hc, cleanup
}

// TestDefaultHealthCheckConfig 测试默认配置.
func TestDefaultHealthCheckConfig(t *testing.T) {
	cfg := DefaultHealthCheckConfig()

	if cfg.CPUWarningThreshold != 70 {
		t.Errorf("CPU 警告阈值应为 70，实际为 %.0f", cfg.CPUWarningThreshold)
	}
	if cfg.CPUCriticalThreshold != 90 {
		t.Errorf("CPU 严重阈值应为 90，实际为 %.0f", cfg.CPUCriticalThreshold)
	}
	if cfg.MemWarningThreshold != 80 {
		t.Errorf("内存警告阈值应为 80，实际为 %.0f", cfg.MemWarningThreshold)
	}
	if cfg.MemCriticalThreshold != 95 {
		t.Errorf("内存严重阈值应为 95，实际为 %.0f", cfg.MemCriticalThreshold)
	}
	if cfg.DiskWarningThreshold != 80 {
		t.Errorf("磁盘警告阈值应为 80，实际为 %.0f", cfg.DiskWarningThreshold)
	}
	if cfg.TempWarningThreshold != 70 {
		t.Errorf("温度警告阈值应为 70，实际为 %d", cfg.TempWarningThreshold)
	}
	if len(cfg.PingHosts) == 0 {
		t.Error("默认 Ping 主机列表不应为空")
	}
	if len(cfg.CoreServices) == 0 {
		t.Error("默认核心服务列表不应为空")
	}
	if !cfg.ParallelCheck {
		t.Error("默认应启用并行检查")
	}
}

// TestNewSystemHealthChecker 测试创建健康检查器.
func TestNewSystemHealthChecker(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	if hc == nil {
		t.Fatal("健康检查器不应为 nil")
	}

	// 默认应注册 7 个内置检查器
	expectedCheckers := []string{"cpu", "memory", "disk", "network", "services", "temperature", "load"}
	for _, name := range expectedCheckers {
		_, err := hc.CheckComponent(context.Background(), name)
		if err != nil {
			t.Errorf("内置检查器 %s 注册失败：%v", name, err)
		}
	}
}

// TestNewSystemHealthCheckerWithNilConfig 测试 nil 配置使用默认值.
func TestNewSystemHealthCheckerWithNilConfig(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	// 使用 nil 配置创建
	hc2 := NewSystemHealthChecker(hc.monitor, nil)
	if hc2 == nil {
		t.Fatal("使用 nil 配置创建不应失败")
	}
	if hc2.config == nil {
		t.Fatal("配置不应为 nil（应使用默认配置）")
	}
}

// TestNewSystemHealthCheckerWithCustomConfig 测试自定义配置.
func TestNewSystemHealthCheckerWithCustomConfig(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	cfg := &HealthCheckConfig{
		CPUWarningThreshold:    50,
		CPUCriticalThreshold:   80,
		MemWarningThreshold:    60,
		MemCriticalThreshold:   90,
		DiskWarningThreshold:   70,
		DiskCriticalThreshold:  95,
		TempWarningThreshold:   60,
		TempCriticalThreshold:  80,
		LoadWarningMultiplier:  0.5,
		LoadCriticalMultiplier: 1.5,
		PingHosts:              []string{"127.0.0.1"},
		PingTimeout:            2,
		CoreServices:           []string{"sshd"},
		ParallelCheck:          false,
	}

	hc2 := NewSystemHealthChecker(hc.monitor, cfg)
	if hc2.config.CPUWarningThreshold != 50 {
		t.Errorf("自定义 CPU 警告阈值应为 50，实际为 %.0f", hc2.config.CPUWarningThreshold)
	}
}

// TestCheckAll 测试全量健康检查.
func TestCheckAll(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	report := hc.CheckAll(context.Background())

	if report == nil {
		t.Fatal("健康报告不应为 nil")
	}

	if report.Hostname == "" {
		t.Error("主机名不应为空")
	}

	if report.CheckedAt.IsZero() {
		t.Error("检查时间不应为零值")
	}

	if report.Duration == "" {
		t.Error("检查耗时不应为空")
	}

	if len(report.Components) == 0 {
		t.Error("组件列表不应为空")
	}

	// 验证 Summary 存在
	if report.Summary == nil {
		t.Error("Summary 不应为 nil")
	}

	// 验证状态有效
	validStatuses := map[HealthStatus]bool{
		StatusHealthy:   true,
		StatusDegraded:  true,
		StatusUnhealthy: true,
		StatusCritical:  true,
	}
	if !validStatuses[report.Status] {
		t.Errorf("无效的健康状态：%s", report.Status)
	}

	if report.Message == "" {
		t.Error("状态描述不应为空")
	}

	t.Logf("整体状态：%s - %s", report.Status, report.Message)
	t.Logf("组件数量：%d", len(report.Components))

	for _, comp := range report.Components {
		t.Logf("  %s: %s - %s", comp.Name, comp.Status, comp.Message)
	}
}

// TestCheckComponent 测试单组件检查.
func TestCheckComponent(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	tests := []struct {
		name        string
		expectValid bool
	}{
		{"cpu", true},
		{"memory", true},
		{"disk", true},
		{"network", true},
		{"services", true},
		{"temperature", true},
		{"load", true},
		{"unknown_component", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp, err := hc.CheckComponent(context.Background(), tt.name)
			if tt.expectValid {
				if err != nil {
					t.Fatalf("检查组件 %s 失败：%v", tt.name, err)
				}
				if comp == nil {
					t.Fatalf("组件 %s 结果不应为 nil", tt.name)
				}
				if comp.Name != tt.name {
					t.Errorf("组件名称应为 %s，实际为 %s", tt.name, comp.Name)
				}
				if comp.CheckedAt.IsZero() {
					t.Error("检查时间不应为零值")
				}
				if comp.Duration == "" {
					t.Error("检查耗时不应为空")
				}
				t.Logf("%s: %s - %s", comp.Name, comp.Status, comp.Message)
			} else {
				if err == nil {
					t.Errorf("未知组件 %s 应返回错误", tt.name)
				}
			}
		})
	}
}

// TestRegisterChecker 测试自定义检查器注册.
func TestRegisterChecker(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	// 注册自定义检查器
	hc.RegisterChecker("custom", func(ctx context.Context) *ComponentHealth {
		return &ComponentHealth{
			Name:      "custom",
			Status:    StatusHealthy,
			Message:   "自定义组件正常",
			CheckedAt: timeNow(),
			Duration:  "0s",
		}
	})

	comp, err := hc.CheckComponent(context.Background(), "custom")
	if err != nil {
		t.Fatalf("自定义检查器调用失败：%v", err)
	}
	if comp.Status != StatusHealthy {
		t.Errorf("自定义组件状态应为 healthy，实际为 %s", comp.Status)
	}

	// 注册一个会返回 Critical 的自定义检查器
	hc.RegisterChecker("critical_custom", func(ctx context.Context) *ComponentHealth {
		return &ComponentHealth{
			Name:      "critical_custom",
			Status:    StatusCritical,
			Message:   "自定义组件异常",
			CheckedAt: timeNow(),
			Duration:  "0s",
		}
	})

	// CheckAll 应包含自定义检查器
	report := hc.CheckAll(context.Background())
	found := false
	for _, comp := range report.Components {
		if comp.Name == "critical_custom" {
			found = true
			if comp.Status != StatusCritical {
				t.Errorf("自定义组件应为 critical，实际为 %s", comp.Status)
			}
		}
	}
	if !found {
		t.Error("CheckAll 应包含自定义检查器结果")
	}
}

// TestCheckCPU 测试 CPU 检查.
func TestCheckCPU(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	comp := hc.checkCPU(context.Background())

	if comp.Name != "cpu" {
		t.Errorf("组件名称应为 cpu，实际为 %s", comp.Name)
	}

	if comp.Details == nil {
		t.Error("CPU 详情不应为 nil")
	}

	// 验证 Details 包含必要字段
	if _, ok := comp.Details["usage"]; !ok {
		t.Error("CPU 详情应包含 usage 字段")
	}
	if _, ok := comp.Details["cores"]; !ok {
		t.Error("CPU 详情应包含 cores 字段")
	}

	t.Logf("CPU 状态：%s - %s", comp.Status, comp.Message)
}

// TestCheckMemory 测试内存检查.
func TestCheckMemory(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	comp := hc.checkMemory(context.Background())

	if comp.Name != "memory" {
		t.Errorf("组件名称应为 memory，实际为 %s", comp.Name)
	}

	if comp.Details == nil {
		t.Error("内存详情不应为 nil")
	}

	requiredFields := []string{"usage", "total", "used", "free", "swapUsage"}
	for _, field := range requiredFields {
		if _, ok := comp.Details[field]; !ok {
			t.Errorf("内存详情应包含 %s 字段", field)
		}
	}

	t.Logf("内存状态：%s - %s", comp.Status, comp.Message)
}

// TestCheckDisk 测试磁盘检查.
func TestCheckDisk(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	comp := hc.checkDisk(context.Background())

	if comp.Name != "disk" {
		t.Errorf("组件名称应为 disk，实际为 %s", comp.Name)
	}

	if comp.Details == nil {
		t.Error("磁盘详情不应为 nil")
	}

	t.Logf("磁盘状态：%s - %s", comp.Status, comp.Message)
}

// TestCheckNetwork 测试网络检查.
func TestCheckNetwork(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	comp := hc.checkNetwork(context.Background())

	if comp.Name != "network" {
		t.Errorf("组件名称应为 network，实际为 %s", comp.Name)
	}

	if comp.Details == nil {
		t.Error("网络详情不应为 nil")
	}

	if _, ok := comp.Details["hosts"]; !ok {
		t.Error("网络详情应包含 hosts 字段")
	}
	if _, ok := comp.Details["reachable"]; !ok {
		t.Error("网络详情应包含 reachable 字段")
	}

	t.Logf("网络状态：%s - %s", comp.Status, comp.Message)
}

// TestCheckNetworkLocalhost 测试本地网络连通性.
func TestCheckNetworkLocalhost(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	// 使用本地地址配置
	hc.config.PingHosts = []string{"127.0.0.1"}
	comp := hc.checkNetwork(context.Background())

	// localhost 应该可达
	if comp.Status == StatusCritical {
		t.Errorf("本地网络应可达，但状态为 %s: %s", comp.Status, comp.Message)
	}
}

// TestCheckServices 测试服务检查.
func TestCheckServices(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	comp := hc.checkServices(context.Background())

	if comp.Name != "services" {
		t.Errorf("组件名称应为 services，实际为 %s", comp.Name)
	}

	t.Logf("服务状态：%s - %s", comp.Status, comp.Message)
}

// TestCheckServicesEmpty 测试空服务列表.
func TestCheckServicesEmpty(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	hc.config.CoreServices = []string{}
	comp := hc.checkServices(context.Background())

	if comp.Status != StatusHealthy {
		t.Errorf("空服务列表应为 healthy，实际为 %s", comp.Status)
	}
}

// TestCheckTemperature 测试温度检查.
func TestCheckTemperature(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	comp := hc.checkTemperature(context.Background())

	if comp.Name != "temperature" {
		t.Errorf("组件名称应为 temperature，实际为 %s", comp.Name)
	}

	t.Logf("温度状态：%s - %s", comp.Status, comp.Message)
}

// TestCheckLoad 测试负载检查.
func TestCheckLoad(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	comp := hc.checkLoad(context.Background())

	if comp.Name != "load" {
		t.Errorf("组件名称应为 load，实际为 %s", comp.Name)
	}

	if comp.Details == nil {
		t.Error("负载详情不应为 nil")
	}

	requiredFields := []string{"load1", "load5", "load15", "cores"}
	for _, field := range requiredFields {
		if _, ok := comp.Details[field]; !ok {
			t.Errorf("负载详情应包含 %s 字段", field)
		}
	}

	t.Logf("负载状态：%s - %s", comp.Status, comp.Message)
}

// TestGetOverallStatus 测试快速状态获取.
func TestGetOverallStatus(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	status := hc.GetOverallStatus(context.Background())

	validStatuses := map[HealthStatus]bool{
		StatusHealthy:   true,
		StatusDegraded:  true,
		StatusUnhealthy: true,
		StatusCritical:  true,
	}
	if !validStatuses[status] {
		t.Errorf("无效的健康状态：%s", status)
	}

	t.Logf("整体状态：%s", status)
}

// TestHealthCheckHandler 测试 HTTP 处理器.
func TestHealthCheckHandler(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	// 创建 Gin 引擎
	router := gin.New()
	router.GET("/health", hc.HealthCheckHandler)

	// 发送请求
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("HTTP 状态码应为 200 或 503，实际为 %d", w.Code)
	}

	// 解析 JSON 响应
	var report SystemHealth
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatalf("解析 JSON 响应失败：%v", err)
	}

	// 验证响应结构
	if report.CheckedAt.IsZero() {
		t.Error("报告中检查时间不应为零值")
	}
	if len(report.Components) == 0 {
		t.Error("报告中组件列表不应为空")
	}

	t.Logf("HTTP 响应状态码：%d", w.Code)
	t.Logf("系统健康状态：%s", report.Status)
}

// TestHealthCheckHandlerDegraded 测试降级状态的 HTTP 响应.
func TestHealthCheckHandlerDegraded(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	// 覆盖所有内置检查器，确保整体状态可控
	// CPU 返回 Degraded，其余全部 Healthy
	hc.RegisterChecker("cpu", func(ctx context.Context) *ComponentHealth {
		return &ComponentHealth{
			Name:      "cpu",
			Status:    StatusDegraded,
			Message:   "CPU 使用率偏高",
			CheckedAt: timeNow(),
			Duration:  "0s",
		}
	})
	healthyCheckers := []string{"memory", "disk", "network", "services", "temperature", "load"}
	for _, name := range healthyCheckers {
		n := name // capture loop var
		hc.RegisterChecker(n, func(ctx context.Context) *ComponentHealth {
			return &ComponentHealth{
				Name:      n,
				Status:    StatusHealthy,
				Message:   n + " 正常",
				CheckedAt: timeNow(),
				Duration:  "0s",
			}
		})
	}

	router := gin.New()
	router.GET("/health", hc.HealthCheckHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Degraded 应返回 200（系统仍然可用）
	if w.Code != http.StatusOK {
		t.Errorf("Degraded 状态 HTTP 应返回 200，实际为 %d", w.Code)
	}
}

// TestHealthCheckHandlerCritical 测试严重状态的 HTTP 响应.
func TestHealthCheckHandlerCritical(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	// 覆盖所有内置检查器，确保整体状态可控
	hc.RegisterChecker("cpu", func(ctx context.Context) *ComponentHealth {
		return &ComponentHealth{
			Name:      "cpu",
			Status:    StatusCritical,
			Message:   "CPU 使用率严重过高",
			CheckedAt: timeNow(),
			Duration:  "0s",
		}
	})
	healthyCheckers := []string{"memory", "disk", "network", "services", "temperature", "load"}
	for _, name := range healthyCheckers {
		n := name
		hc.RegisterChecker(n, func(ctx context.Context) *ComponentHealth {
			return &ComponentHealth{
				Name:      n,
				Status:    StatusHealthy,
				Message:   n + " 正常",
				CheckedAt: timeNow(),
				Duration:  "0s",
			}
		})
	}

	router := gin.New()
	router.GET("/health", hc.HealthCheckHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Critical 应返回 503
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Critical 状态 HTTP 应返回 503，实际为 %d", w.Code)
	}
}

// TestHealthCheckHandlerUnhealthy 测试不健康状态的 HTTP 响应.
func TestHealthCheckHandlerUnhealthy(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	// 覆盖所有内置检查器，确保整体状态可控
	hc.RegisterChecker("cpu", func(ctx context.Context) *ComponentHealth {
		return &ComponentHealth{
			Name:      "cpu",
			Status:    StatusUnhealthy,
			Message:   "CPU 异常",
			CheckedAt: timeNow(),
			Duration:  "0s",
		}
	})
	healthyCheckers := []string{"memory", "disk", "network", "services", "temperature", "load"}
	for _, name := range healthyCheckers {
		n := name
		hc.RegisterChecker(n, func(ctx context.Context) *ComponentHealth {
			return &ComponentHealth{
				Name:      n,
				Status:    StatusHealthy,
				Message:   n + " 正常",
				CheckedAt: timeNow(),
				Duration:  "0s",
			}
		})
	}

	router := gin.New()
	router.GET("/health", hc.HealthCheckHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Unhealthy 应返回 503
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Unhealthy 状态 HTTP 应返回 503，实际为 %d", w.Code)
	}
}

// TestHealthCheckHandlerJSON 测试 JSON 响应格式.
func TestHealthCheckHandlerJSON(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	router := gin.New()
	router.GET("/health", hc.HealthCheckHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 验证是合法 JSON
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("响应不是合法 JSON：%v", err)
	}

	// 验证必要字段
	requiredFields := []string{"status", "message", "hostname", "checkedAt", "duration", "components", "summary"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("JSON 响应缺少字段：%s", field)
		}
	}
}

// TestSequentialCheck 测试顺序检查.
func TestSequentialCheck(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	hc.config.ParallelCheck = false

	report := hc.CheckAll(context.Background())

	if report == nil {
		t.Fatal("顺序检查不应返回 nil")
	}

	if len(report.Components) == 0 {
		t.Error("顺序检查组件列表不应为空")
	}
}

// TestStatusMessage 测试状态消息.
func TestStatusMessage(t *testing.T) {
	tests := []struct {
		status   HealthStatus
		expected string
	}{
		{StatusHealthy, "系统运行正常"},
		{StatusDegraded, "系统部分功能降级"},
		{StatusUnhealthy, "系统不健康，多个组件异常"},
		{StatusCritical, "系统严重异常，需要立即处理"},
		{HealthStatus("unknown"), "未知状态"},
	}

	for _, tt := range tests {
		msg := statusMessage(tt.status)
		if msg != tt.expected {
			t.Errorf("statusMessage(%s) = %q, 期望 %q", tt.status, msg, tt.expected)
		}
	}
}

// TestPingHost 测试网络连通性检测函数.
func TestPingHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		timeout  int
		expectOk bool
	}{
		{"localhost", "127.0.0.1", 2, true},
		{"dns_google", "8.8.8.8", 5, true},
		{"invalid_host", "255.255.255.255", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 这些测试依赖网络环境，只记录日志
			result := pingHost(tt.host, tt.timeout)
			t.Logf("pingHost(%s, %d) = %v (期望 %v)", tt.host, tt.timeout, result, tt.expectOk)
			// 不做硬断言，因为 CI 环境网络可能不同
		})
	}
}

// TestCheckServiceStatus 测试服务状态检查函数.
func TestCheckServiceStatus(t *testing.T) {
	// 通常不存在的服务应返回 unknown
	status := checkServiceStatus(context.Background(), "nonexistent_service_xyz")
	if status != "unknown" {
		t.Logf("不存在的服务状态：%s（可能因环境而异）", status)
	}
}

// TestHealthStatusConstants 测试健康状态常量.
func TestHealthStatusConstants(t *testing.T) {
	if StatusHealthy != "healthy" {
		t.Errorf("StatusHealthy 应为 'healthy'，实际为 %s", StatusHealthy)
	}
	if StatusDegraded != "degraded" {
		t.Errorf("StatusDegraded 应为 'degraded'，实际为 %s", StatusDegraded)
	}
	if StatusUnhealthy != "unhealthy" {
		t.Errorf("StatusUnhealthy 应为 'unhealthy'，实际为 %s", StatusUnhealthy)
	}
	if StatusCritical != "critical" {
		t.Errorf("StatusCritical 应为 'critical'，实际为 %s", StatusCritical)
	}
}

// TestStatusOrder 测试状态严重程度排序.
func TestStatusOrder(t *testing.T) {
	if statusOrder[StatusHealthy] >= statusOrder[StatusDegraded] {
		t.Error("Healthy 应比 Degraded 严重程度低")
	}
	if statusOrder[StatusDegraded] >= statusOrder[StatusUnhealthy] {
		t.Error("Degraded 应比 Unhealthy 严重程度低")
	}
	if statusOrder[StatusUnhealthy] >= statusOrder[StatusCritical] {
		t.Error("Unhealthy 应比 Critical 严重程度低")
	}
}

// TestCalculateOverallStatus 测试整体状态计算.
func TestCalculateOverallStatus(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	tests := []struct {
		name       string
		components []*ComponentHealth
		expected   HealthStatus
	}{
		{
			name:       "空组件列表",
			components: []*ComponentHealth{},
			expected:   StatusHealthy,
		},
		{
			name: "全部健康",
			components: []*ComponentHealth{
				{Status: StatusHealthy},
				{Status: StatusHealthy},
			},
			expected: StatusHealthy,
		},
		{
			name: "存在降级",
			components: []*ComponentHealth{
				{Status: StatusHealthy},
				{Status: StatusDegraded},
			},
			expected: StatusDegraded,
		},
		{
			name: "存在不健康",
			components: []*ComponentHealth{
				{Status: StatusHealthy},
				{Status: StatusUnhealthy},
			},
			expected: StatusUnhealthy,
		},
		{
			name: "存在严重",
			components: []*ComponentHealth{
				{Status: StatusHealthy},
				{Status: StatusDegraded},
				{Status: StatusCritical},
			},
			expected: StatusCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hc.calculateOverallStatus(tt.components)
			if result != tt.expected {
				t.Errorf("期望 %s，实际 %s", tt.expected, result)
			}
		})
	}
}

// TestBuildSummary 测试 Summary 构建.
func TestBuildSummary(t *testing.T) {
	hc, cleanup := newTestHealthChecker(t)
	defer cleanup()

	components := []*ComponentHealth{
		{Status: StatusHealthy},
		{Status: StatusHealthy},
		{Status: StatusDegraded},
		{Status: StatusCritical},
	}

	summary := hc.buildSummary(components)

	if summary[string(StatusHealthy)] != 2 {
		t.Errorf("Healthy 数量应为 2，实际为 %d", summary[string(StatusHealthy)])
	}
	if summary[string(StatusDegraded)] != 1 {
		t.Errorf("Degraded 数量应为 1，实际为 %d", summary[string(StatusDegraded)])
	}
	if summary[string(StatusCritical)] != 1 {
		t.Errorf("Critical 数量应为 1，实际为 %d", summary[string(StatusCritical)])
	}
	if summary[string(StatusUnhealthy)] != 0 {
		t.Errorf("Unhealthy 数量应为 0，实际为 %d", summary[string(StatusUnhealthy)])
	}
}

// TestComponentHealthDetails 组件详情字段测试.
func TestComponentHealthDetails(t *testing.T) {
	ch := &ComponentHealth{
		Name:      "test",
		Status:    StatusHealthy,
		Message:   "测试组件",
		CheckedAt: timeNow(),
		Duration:  "10ms",
		Details: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}

	// JSON 序列化/反序列化验证
	data, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("JSON 序列化失败：%v", err)
	}

	var decoded ComponentHealth
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON 反序列化失败：%v", err)
	}

	if decoded.Name != ch.Name {
		t.Errorf("名称应为 %s，实际为 %s", ch.Name, decoded.Name)
	}
	if decoded.Status != ch.Status {
		t.Errorf("状态应为 %s，实际为 %s", ch.Status, decoded.Status)
	}
}

// TestSystemHealthJSON 序列化/反序列化验证.
func TestSystemHealthJSON(t *testing.T) {
	health := &SystemHealth{
		Status:    StatusHealthy,
		Message:   "系统正常",
		Hostname:  "test-host",
		CheckedAt: timeNow(),
		Duration:  "100ms",
		Components: []*ComponentHealth{
			{
				Name:      "cpu",
				Status:    StatusHealthy,
				Message:   "CPU 正常",
				CheckedAt: timeNow(),
				Duration:  "10ms",
			},
		},
		Summary: map[string]int{
			"healthy":  1,
			"degraded": 0,
		},
	}

	data, err := json.Marshal(health)
	if err != nil {
		t.Fatalf("JSON 序列化失败：%v", err)
	}

	var decoded SystemHealth
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON 反序列化失败：%v", err)
	}

	if decoded.Status != StatusHealthy {
		t.Errorf("状态应为 healthy，实际为 %s", decoded.Status)
	}
	if decoded.Hostname != "test-host" {
		t.Errorf("主机名应为 test-host，实际为 %s", decoded.Hostname)
	}
	if len(decoded.Components) != 1 {
		t.Errorf("组件数量应为 1，实际为 %d", len(decoded.Components))
	}
}

// timeNow 返回当前时间（辅助函数）.
func timeNow() time.Time {
	return time.Now()
}
