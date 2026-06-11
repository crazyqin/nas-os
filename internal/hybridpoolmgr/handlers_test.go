// Package hybridpoolmgr 混合存储池 REST API 处理器测试
package hybridpoolmgr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestHandlers 创建测试用处理器.
func newTestHandlers(t *testing.T) (*Handlers, *gin.Engine) {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	mgr, err := NewManager(logger, t.TempDir())
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	h := NewHandlers(mgr, logger)

	router := gin.New()
	h.RegisterRoutes(router.Group(""))

	return h, router
}

// createTestPool 通过 API 创建测试池.
func createTestPool(t *testing.T, router *gin.Engine, name string) {
	t.Helper()
	body, _ := json.Marshal(CreatePoolRequest{
		Name:        name,
		Description: "测试池",
		NVMEDevices: []string{"/dev/nvme0n1"},
		SSDDevices:  []string{"/dev/sda"},
		HDDDevices:  []string{"/dev/sdb"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("创建测试池失败: %d", w.Code)
	}
}

// ========== Handlers 结构测试 ==========

// TestHandlers_Struct 测试处理器结构体.
func TestHandlers_Struct(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr, _ := NewManager(logger, t.TempDir())
	h := &Handlers{manager: mgr, logger: logger}
	if h == nil {
		t.Error("处理器不应为 nil")
	}
}

// TestNewHandlers 测试创建处理器.
func TestNewHandlers(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr, _ := NewManager(logger, t.TempDir())
	h := NewHandlers(mgr, logger)
	if h == nil {
		t.Fatal("处理器不应为 nil")
	}
	if h.manager != mgr {
		t.Error("manager 引用不匹配")
	}
}

// ========== 池管理路由测试 ==========

// TestHandlers_RegisterRoutes 测试路由注册.
func TestHandlers_RegisterRoutes(t *testing.T) {
	_, router := newTestHandlers(t)

	routes := router.Routes()
	if len(routes) == 0 {
		t.Error("期望注册路由")
	}

	// 检查关键路由
	expectedRoutes := []string{
		"/hybrid-pools",
		"/hybrid-pools/:name",
		"/hybrid-pools/:name/devices",
		"/hybrid-pools/:name/io-stats",
		"/hybrid-pools/:name/heat-analysis",
		"/hybrid-pools/:name/tiering/run",
		"/hybrid-pools/:name/rebalance/run",
		"/hybrid-pools/:name/health",
		"/hybrid-pools/:name/alerts",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	for _, expected := range expectedRoutes {
		if !routeMap[expected] {
			t.Errorf("期望路由 %s 已注册", expected)
		}
	}
}

// TestHandlers_ListPools 测试列出池.
func TestHandlers_ListPools(t *testing.T) {
	_, router := newTestHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hybrid-pools", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["code"].(float64) != 0 {
		t.Errorf("期望 code 为 0，实际为 %v", resp["code"])
	}
}

// TestHandlers_CreatePool 测试创建池.
func TestHandlers_CreatePool(t *testing.T) {
	_, router := newTestHandlers(t)

	body, _ := json.Marshal(CreatePoolRequest{
		Name:        "api-pool",
		Description: "通过 API 创建",
		NVMEDevices: []string{"/dev/nvme0n1"},
		SSDDevices:  []string{"/dev/sda"},
		HDDDevices:  []string{"/dev/sdb"},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 201，实际为 %d", w.Code)
	}
}

// TestHandlers_CreatePool_Invalid 测试无效请求.
func TestHandlers_CreatePool_Invalid(t *testing.T) {
	_, router := newTestHandlers(t)

	// 缺少必需字段
	body := `{"description": "没有名称"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，实际为 %d", w.Code)
	}
}

// TestHandlers_GetPool 测试获取池.
func TestHandlers_GetPool(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hybrid-pools/test-pool", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_GetPool_NotFound 测试获取不存在的池.
func TestHandlers_GetPool_NotFound(t *testing.T) {
	_, router := newTestHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hybrid-pools/non-existent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404，实际为 %d", w.Code)
	}
}

// TestHandlers_DeletePool 测试删除池.
func TestHandlers_DeletePool(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/hybrid-pools/test-pool", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_DeletePool_NotFound 测试删除不存在的池.
func TestHandlers_DeletePool_NotFound(t *testing.T) {
	_, router := newTestHandlers(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/hybrid-pools/non-existent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("期望状态码 500，实际为 %d", w.Code)
	}
}

// ========== 设备管理路由测试 ==========

// TestHandlers_AddDevice 测试添加设备.
func TestHandlers_AddDevice(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	body, _ := json.Marshal(AddDeviceRequest{
		DevicePath: "/dev/sdd",
		Tier:       TierHDD,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools/test-pool/devices", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_RemoveDevice 测试移除设备.
func TestHandlers_RemoveDevice(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/hybrid-pools/test-pool/devices?device=/dev/sdb", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_RemoveDevice_NoDevice 测试移除设备时缺少参数.
func TestHandlers_RemoveDevice_NoDevice(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/hybrid-pools/test-pool/devices", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400，实际为 %d", w.Code)
	}
}

// ========== IO 统计路由测试 ==========

// TestHandlers_GetIOStats 测试获取 IO 统计.
func TestHandlers_GetIOStats(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hybrid-pools/test-pool/io-stats", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_AnalyzeHeat 测试热度分析.
func TestHandlers_AnalyzeHeat(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hybrid-pools/test-pool/heat-analysis?top=5", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_RecordIO 测试记录 IO.
func TestHandlers_RecordIO(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	body, _ := json.Marshal(map[string]interface{}{
		"blockId": "block-001",
		"path":    "/data/file.dat",
		"tier":    "ssd",
		"size":    4096,
		"isRead":  true,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools/test-pool/io", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_GetBlockHeat 测试获取块热度.
func TestHandlers_GetBlockHeat(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	// 先记录 IO
	body, _ := json.Marshal(map[string]interface{}{
		"blockId": "block-001",
		"path":    "/data/file.dat",
		"tier":    "ssd",
		"size":    4096,
		"isRead":  true,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools/test-pool/io", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 获取块热度
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/hybrid-pools/test-pool/blocks/block-001/heat", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// ========== 分层路由测试 ==========

// TestHandlers_RunTiering 测试执行分层.
func TestHandlers_RunTiering(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools/test-pool/tiering/run", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_UpdateTieringConfig 测试更新分层配置.
func TestHandlers_UpdateTieringConfig(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	body, _ := json.Marshal(TieringConfig{
		Enabled:      true,
		HotThreshold: 2000,
		ColdAgeDays:  60,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/hybrid-pools/test-pool/tiering", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// ========== 重平衡路由测试 ==========

// TestHandlers_RunRebalance 测试执行重平衡.
func TestHandlers_RunRebalance(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools/test-pool/rebalance/run", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_UpdateRebalancePolicy 测试更新重平衡策略.
func TestHandlers_UpdateRebalancePolicy(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	body, _ := json.Marshal(RebalancePolicy{
		Enabled:          true,
		ThresholdPercent: 20.0,
		MaxMigrateMBps:   500,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/hybrid-pools/test-pool/rebalance", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// ========== 健康监控路由测试 ==========

// TestHandlers_CheckHealth 测试健康检查.
func TestHandlers_CheckHealth(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hybrid-pools/test-pool/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_GetAlerts 测试获取告警.
func TestHandlers_GetAlerts(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hybrid-pools/test-pool/alerts", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_AddAlert 测试添加告警.
func TestHandlers_AddAlert(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	body, _ := json.Marshal(map[string]string{
		"device":  "/dev/sda",
		"message": "设备温度过高",
		"level":   "warning",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools/test-pool/alerts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// TestHandlers_ResolveAlert 测试解决告警.
func TestHandlers_ResolveAlert(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	// 先添加告警
	body, _ := json.Marshal(map[string]string{
		"device":  "/dev/sda",
		"message": "设备温度过高",
		"level":   "warning",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/hybrid-pools/test-pool/alerts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// 获取告警列表
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/hybrid-pools/test-pool/alerts", nil)
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].([]interface{})
	if len(data) == 0 {
		t.Skip("没有告警可解决")
		return
	}

	alert := data[0].(map[string]interface{})
	alertID := alert["id"].(string)

	// 解决告警
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/hybrid-pools/test-pool/alerts/"+alertID+"/resolve", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际为 %d", w.Code)
	}
}

// ========== JSON 序列化测试 ==========

// TestHandlers_JSON 测试 JSON 响应格式.
func TestHandlers_JSON(t *testing.T) {
	_, router := newTestHandlers(t)
	createTestPool(t, router, "test-pool")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/hybrid-pools/test-pool", nil)
	router.ServeHTTP(w, req)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	// 检查响应结构
	if _, ok := resp["code"]; !ok {
		t.Error("响应应包含 code 字段")
	}
	if _, ok := resp["message"]; !ok {
		t.Error("响应应包含 message 字段")
	}
	if _, ok := resp["data"]; !ok {
		t.Error("响应应包含 data 字段")
	}
}
