// Package gpuscheduler 单元测试
package gpuscheduler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// setupTestRouter 创建测试路由器
func setupTestRouter() (*gin.Engine, *Scheduler) {
	gin.SetMode(gin.TestMode)

	logger, _ := zap.NewDevelopment()
	scheduler := NewScheduler(logger)

	// 加载模拟设备
	if err := scheduler.loadMockDevices(); err != nil {
		panic(err)
	}

	handlers := NewHandlers(scheduler, logger)

	r := gin.New()
	api := r.Group("/api")
	handlers.RegisterRoutes(api)

	return r, scheduler
}

// TestListDevices 测试列出 GPU 设备
func TestListDevices(t *testing.T) {
	r, _ := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/gpuscheduler/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d", resp.Code)
	}

	// 验证返回设备列表
	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	if len(data) != 2 {
		t.Fatalf("期望 2 个设备，实际 %d", len(data))
	}
}

// TestGetDevice 测试获取指定 GPU 设备
func TestGetDevice(t *testing.T) {
	r, _ := setupTestRouter()

	// 测试存在的设备
	req, _ := http.NewRequest("GET", "/api/gpuscheduler/devices/GPU-MOCK-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d", resp.Code)
	}

	// 测试不存在的设备
	req, _ = http.NewRequest("GET", "/api/gpuscheduler/devices/GPU-NOT-EXIST", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("期望状态码 404，实际 %d", w.Code)
	}
}

// TestAllocateAndRelease 测试分配和释放 GPU 资源
func TestAllocateAndRelease(t *testing.T) {
	r, scheduler := setupTestRouter()

	// 测试分配
	allocReq := AllocateRequest{
		ContainerID:   "container-test-001",
		ContainerName: "test-container",
		MemoryMiB:     4096,
		Priority:      PriorityHigh,
	}

	body, _ := json.Marshal(allocReq)
	req, _ := http.NewRequest("POST", "/api/gpuscheduler/allocate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望状态码 201，实际 %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != 0 {
		t.Fatalf("期望 code 0，实际 %d", resp.Code)
	}

	// 验证分配记录
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	allocationID, ok := data["id"].(string)
	if !ok || allocationID == "" {
		t.Fatal("分配 ID 为空")
	}

	// 验证设备已分配显存
	devices := scheduler.ListDevices()
	var allocatedDevice *GPUDevice
	for _, d := range devices {
		if d.MemoryUsed > 0 {
			allocatedDevice = d
			break
		}
	}

	if allocatedDevice == nil {
		t.Fatal("未找到已分配的设备")
	}

	if allocatedDevice.MemoryUsed != 4096 {
		t.Fatalf("期望已用显存 4096 MiB，实际 %d MiB", allocatedDevice.MemoryUsed)
	}

	// 测试释放
	req, _ = http.NewRequest("DELETE", "/api/gpuscheduler/allocate/"+allocationID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}

	// 验证显存已释放
	for _, d := range devices {
		if d.ID == allocatedDevice.ID {
			if d.MemoryUsed != 0 {
				t.Fatalf("期望已用显存 0 MiB，实际 %d MiB", d.MemoryUsed)
			}
		}
	}
}

// TestAllocateInvalidRequest 测试无效分配请求
func TestAllocateInvalidRequest(t *testing.T) {
	r, _ := setupTestRouter()

	// 缺少必填字段
	invalidReq := map[string]interface{}{
		"container_id": "test",
		// 缺少 memory_mib
	}

	body, _ := json.Marshal(invalidReq)
	req, _ := http.NewRequest("POST", "/api/gpuscheduler/allocate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望状态码 400，实际 %d", w.Code)
	}

	// memory_mib 为 0
	invalidReq2 := map[string]interface{}{
		"container_id": "test",
		"memory_mib":   0,
	}

	body, _ = json.Marshal(invalidReq2)
	req, _ = http.NewRequest("POST", "/api/gpuscheduler/allocate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望状态码 400，实际 %d", w.Code)
	}
}

// TestGetStats 测试获取统计信息
func TestGetStats(t *testing.T) {
	r, scheduler := setupTestRouter()

	// 先分配一些资源
	allocReq := AllocateRequest{
		ContainerID: "container-stats-001",
		MemoryMiB:   8192,
		Priority:    PriorityMedium,
	}
	_, err := scheduler.Allocate(allocReq)
	if err != nil {
		t.Fatalf("分配资源失败: %v", err)
	}

	// 获取统计信息
	req, _ := http.NewRequest("GET", "/api/gpuscheduler/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("响应数据格式错误")
	}

	// 验证统计数据
	totalDevices, ok := data["total_devices"].(float64)
	if !ok || totalDevices != 2 {
		t.Fatalf("期望 total_devices 2，实际 %v", data["total_devices"])
	}

	activeAllocations, ok := data["active_allocations"].(float64)
	if !ok || activeAllocations != 1 {
		t.Fatalf("期望 active_allocations 1，实际 %v", data["active_allocations"])
	}
}

// TestUpdatePolicy 测试更新调度策略
func TestUpdatePolicy(t *testing.T) {
	r, scheduler := setupTestRouter()

	// 测试更新策略
	newStrategy := StrategyRoundRobin
	preemption := true
	reserved := 20.0
	overcommit := 1.5
	maxTemp := 90

	updateReq := UpdatePolicyRequest{
		Strategy:          newStrategy,
		PreemptionEnabled: &preemption,
		ReservedPercent:   &reserved,
		OvercommitRatio:   &overcommit,
		MaxTemperature:    &maxTemp,
	}

	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PUT", "/api/gpuscheduler/policy", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望状态码 200，实际 %d", w.Code)
	}

	// 验证策略已更新
	policy := scheduler.GetPolicy()
	if policy.Strategy != StrategyRoundRobin {
		t.Fatalf("期望策略 %s，实际 %s", StrategyRoundRobin, policy.Strategy)
	}
	if !policy.PreemptionEnabled {
		t.Fatal("期望抢占已启用")
	}
	if policy.ReservedPercent != 20.0 {
		t.Fatalf("期望预留百分比 20.0，实际 %f", policy.ReservedPercent)
	}
	if policy.OvercommitRatio != 1.5 {
		t.Fatalf("期望超分配比率 1.5，实际 %f", policy.OvercommitRatio)
	}
	if policy.MaxTemperature != 90 {
		t.Fatalf("期望最大温度 90，实际 %d", policy.MaxTemperature)
	}
}

// TestUpdatePolicyInvalid 测试无效策略更新
func TestUpdatePolicyInvalid(t *testing.T) {
	r, _ := setupTestRouter()

	// 预留百分比超出范围
	reserved := 150.0
	updateReq := UpdatePolicyRequest{
		ReservedPercent: &reserved,
	}

	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PUT", "/api/gpuscheduler/policy", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望状态码 400，实际 %d", w.Code)
	}

	// 超分配比率小于 1.0
	overcommit := 0.5
	updateReq2 := UpdatePolicyRequest{
		OvercommitRatio: &overcommit,
	}

	body, _ = json.Marshal(updateReq2)
	req, _ = http.NewRequest("PUT", "/api/gpuscheduler/policy", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望状态码 400，实际 %d", w.Code)
	}
}

// TestAffinityConstraint 测试亲和性约束
func TestAffinityConstraint(t *testing.T) {
	r, scheduler := setupTestRouter()

	// 测试亲和性约束 - 偏好指定设备
	allocReq := AllocateRequest{
		ContainerID: "container-affinity-001",
		MemoryMiB:   2048,
		Priority:    PriorityMedium,
		Constraint: &AffinityConstraint{
			PreferredDeviceIDs: []string{"GPU-MOCK-002"},
		},
	}

	body, _ := json.Marshal(allocReq)
	req, _ := http.NewRequest("POST", "/api/gpuscheduler/allocate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望状态码 201，实际 %d", w.Code)
	}

	// 验证分配到了指定设备
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	deviceID := data["device_id"].(string)

	if deviceID != "GPU-MOCK-002" {
		t.Fatalf("期望分配到 GPU-MOCK-002，实际 %s", deviceID)
	}

	// 测试排除约束
	allocReq2 := AllocateRequest{
		ContainerID: "container-affinity-002",
		MemoryMiB:   2048,
		Constraint: &AffinityConstraint{
			ExcludedDeviceIDs: []string{"GPU-MOCK-002"},
		},
	}

	body, _ = json.Marshal(allocReq2)
	req, _ = http.NewRequest("POST", "/api/gpuscheduler/allocate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("期望状态码 201，实际 %d", w.Code)
	}

	json.Unmarshal(w.Body.Bytes(), &resp)
	data = resp.Data.(map[string]interface{})
	deviceID = data["device_id"].(string)

	if deviceID == "GPU-MOCK-002" {
		t.Fatal("不应分配到被排除的设备 GPU-MOCK-002")
	}

	// 验证设备统计
	devices := scheduler.ListDevices()
	t.Logf("设备分配统计:")
	for _, d := range devices {
		t.Logf("  %s: 已用 %d MiB, 分配数 %d", d.ID, d.MemoryUsed, len(d.Allocations))
	}
}
