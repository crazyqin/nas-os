package rdmastorageaccel

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager()
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandler(m)
	h.RegisterRoutes(rg)
	return r
}

func TestManagerInitialization(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 验证设备已发现
	devices := m.GetDevices()
	if len(devices) == 0 {
		t.Error("期望发现至少一个 RDMA 设备")
	}

	// 验证默认配置
	config := m.GetConfig()
	if config == nil {
		t.Error("期望有默认配置")
	}
	if config.Protocol != ProtocolRoCEv2 {
		t.Errorf("期望协议为 %s, 实际为 %s", ProtocolRoCEv2, config.Protocol)
	}

	// 验证调优预设
	profiles := m.GetTuningProfiles()
	if len(profiles) == 0 {
		t.Error("期望有默认调优预设")
	}
}

func TestDeviceOperations(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 获取所有设备
	devices := m.GetDevices()
	if len(devices) == 0 {
		t.Fatal("未发现设备")
	}

	// 获取单个设备
	device, err := m.GetDevice(devices[0].ID)
	if err != nil {
		t.Fatalf("获取设备失败: %v", err)
	}
	if device.Name == "" {
		t.Error("设备名称不应为空")
	}
	if device.Status != DeviceStatusActive {
		t.Errorf("期望设备状态为 %s, 实际为 %s", DeviceStatusActive, device.Status)
	}

	// 获取不存在的设备
	_, err = m.GetDevice("non-existent-id")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestConfigOperations(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 获取配置
	config := m.GetConfig()
	if config == nil {
		t.Fatal("配置不应为空")
	}

	// 更新配置
	newConfig := &RDMAConfig{
		ID:                "test-config",
		Protocol:          ProtocolIWARP,
		MTU:               9000,
		CongestionControl: CongestionDCTCP,
		QoS: QoSConfig{
			Enabled:       true,
			Priority:      5,
			ServiceLevel:  0,
			TrafficClass:  0,
			MaxSGLen:      32,
			MaxInlineData: 512,
			Timeout:       10,
			RetryCount:    5,
			RNRRetry:      5,
		},
		NetworkDetection: NetworkDetection{
			Enabled:          true,
			AutoDetectMTU:    true,
			AutoDetectSpeed:  true,
			DetectionTimeout: 30,
		},
		Advanced: AdvancedConfig{
			MaxQueuePairs:     256,
			MaxCQEntries:      65536,
			MaxMRSize:         1073741824,
			UseEventfd:        true,
			NumaAware:         true,
			CompletionVector:  0,
			MaxSendWR:         1024,
			MaxRecvWR:         1024,
			MaxRDMAReadAtomic: 16,
		},
	}

	err := m.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	// 验证更新
	updatedConfig := m.GetConfig()
	if updatedConfig.Protocol != ProtocolIWARP {
		t.Errorf("期望协议为 %s, 实际为 %s", ProtocolIWARP, updatedConfig.Protocol)
	}
	if updatedConfig.MTU != 9000 {
		t.Errorf("期望 MTU 为 9000, 实际为 %d", updatedConfig.MTU)
	}

	// 测试无效协议
	invalidConfig := &RDMAConfig{
		Protocol:          "invalid",
		MTU:               4096,
		CongestionControl: CongestionDCQCN,
		QoS:               QoSConfig{},
		NetworkDetection:  NetworkDetection{},
		Advanced:          AdvancedConfig{},
	}
	err = m.UpdateConfig(invalidConfig)
	if err == nil {
		t.Error("期望返回错误")
	}

	// 测试无效 MTU
	invalidMTUConfig := &RDMAConfig{
		Protocol:          ProtocolRoCEv2,
		MTU:               1000,
		CongestionControl: CongestionDCQCN,
		QoS:               QoSConfig{},
		NetworkDetection:  NetworkDetection{},
		Advanced:          AdvancedConfig{},
	}
	err = m.UpdateConfig(invalidMTUConfig)
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestTargetOperations(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 获取设备
	devices := m.GetDevices()
	if len(devices) == 0 {
		t.Fatal("未发现设备")
	}

	// 创建 iSCSI 目标
	iscsiReq := &StorageTarget{
		Name:       "test-iscsi-target",
		Type:       TargetTypeISCSI,
		DeviceID:   devices[0].ID,
		TargetAddr: "192.168.1.100",
		Port:       3260,
		ISCSISettings: &ISCSISettings{
			TargetIQN:     "iqn.2024-01.com.example:storage.target1",
			InitiatorIQN:  "iqn.2024-01.com.example:host.initiator1",
			HeaderDigest:  true,
			DataDigest:    false,
			ImmediateData: true,
			MaxRecvSegLen: 262144,
			MaxBurstLen:   1048576,
			FirstBurstLen: 262144,
			RDMAEnabled:   true,
		},
		Tags: []string{"production", "database"},
	}

	target, err := m.CreateTarget(iscsiReq)
	if err != nil {
		t.Fatalf("创建 iSCSI 目标失败: %v", err)
	}
	if target.ID == "" {
		t.Error("目标 ID 不应为空")
	}
	if target.Status != TargetStatusActive {
		t.Errorf("期望目标状态为 %s, 实际为 %s", TargetStatusActive, target.Status)
	}

	// 创建 NFS 目标
	nfsReq := &StorageTarget{
		Name:       "test-nfs-target",
		Type:       TargetTypeNFS,
		DeviceID:   devices[0].ID,
		TargetAddr: "192.168.1.101",
		Port:       2049,
		NFSSettings: &NFSSettings{
			Version:      "4.2",
			ExportPath:   "/data/share",
			MountOptions: "rdma,port=20049",
			RDMAEnabled:  true,
			RDMAPort:     20049,
		},
		Tags: []string{"fileserver"},
	}

	nfsTarget, err := m.CreateTarget(nfsReq)
	if err != nil {
		t.Fatalf("创建 NFS 目标失败: %v", err)
	}

	// 获取所有目标
	targets := m.GetTargets()
	if len(targets) != 2 {
		t.Errorf("期望有 2 个目标, 实际有 %d", len(targets))
	}

	// 删除目标
	err = m.DeleteTarget(target.ID)
	if err != nil {
		t.Fatalf("删除目标失败: %v", err)
	}

	// 验证删除
	targets = m.GetTargets()
	if len(targets) != 1 {
		t.Errorf("期望有 1 个目标, 实际有 %d", len(targets))
	}

	// 删除不存在的目标
	err = m.DeleteTarget("non-existent-id")
	if err == nil {
		t.Error("期望返回错误")
	}

	// 清理
	m.DeleteTarget(nfsTarget.ID)
}

func TestMetricsOperations(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 获取设备
	devices := m.GetDevices()
	if len(devices) == 0 {
		t.Fatal("未发现设备")
	}

	// 等待指标收集
	// 注意：实际测试中可能需要等待或触发指标收集

	// 获取指定设备指标
	metrics := m.GetMetrics(devices[0].ID)
	if metrics == nil {
		t.Error("期望有性能指标")
	}

	// 获取不存在设备的指标
	metrics = m.GetMetrics("non-existent-id")
	if metrics != nil {
		t.Error("期望返回 nil")
	}

	// 获取历史指标
	history := m.GetMetricsHistory(10)
	if history == nil {
		t.Error("期望有历史指标")
	}
}

func TestBenchmarkOperations(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 获取设备
	devices := m.GetDevices()
	if len(devices) == 0 {
		t.Fatal("未发现设备")
	}

	// 运行基准测试
	config := &BenchmarkConfig{
		DeviceID:   devices[0].ID,
		TestType:   "bandwidth",
		Duration:   1,
		BlockSize:  4096,
		QueueDepth: 32,
		NumThreads: 4,
		ReadWrite:  "read",
	}

	result, err := m.RunBenchmark(config)
	if err != nil {
		t.Fatalf("运行基准测试失败: %v", err)
	}

	if result.ID == "" {
		t.Error("基准测试结果 ID 不应为空")
	}
	if result.BandwidthMBs <= 0 {
		t.Error("带宽应大于 0")
	}
	if result.LatencyUs <= 0 {
		t.Error("延迟应大于 0")
	}
	if result.IOPS <= 0 {
		t.Error("IOPS 应大于 0")
	}

	// 测试无效设备
	invalidConfig := &BenchmarkConfig{
		DeviceID:   "non-existent-id",
		TestType:   "bandwidth",
		Duration:   1,
		BlockSize:  4096,
		QueueDepth: 32,
		NumThreads: 4,
		ReadWrite:  "read",
	}

	_, err = m.RunBenchmark(invalidConfig)
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestTuningProfileOperations(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 获取调优预设
	profiles := m.GetTuningProfiles()
	if len(profiles) == 0 {
		t.Fatal("未找到调优预设")
	}

	// 查找低延迟预设
	var lowLatencyProfile *TuningProfile
	for i, p := range profiles {
		if p.Type == ProfileLowLatency {
			lowLatencyProfile = &profiles[i]
			break
		}
	}
	if lowLatencyProfile == nil {
		t.Fatal("未找到低延迟预设")
	}

	// 应用预设
	err := m.ApplyTuningProfile(lowLatencyProfile.ID)
	if err != nil {
		t.Fatalf("应用调优预设失败: %v", err)
	}

	// 验证配置已更新
	config := m.GetConfig()
	if config.MTU != lowLatencyProfile.MTU {
		t.Errorf("期望 MTU 为 %d, 实际为 %d", lowLatencyProfile.MTU, config.MTU)
	}
	if config.CongestionControl != lowLatencyProfile.Congestion {
		t.Errorf("期望拥塞控制为 %s, 实际为 %s", lowLatencyProfile.Congestion, config.CongestionControl)
	}

	// 应用不存在的预设
	err = m.ApplyTuningProfile("non-existent-profile")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestHealthCheckOperations(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 获取设备
	devices := m.GetDevices()
	if len(devices) == 0 {
		t.Fatal("未发现设备")
	}

	// 执行健康检查
	result := m.HealthCheck(devices[0].ID, "")
	if result == nil {
		t.Fatal("健康检查结果不应为空")
	}
	if !result.Healthy {
		t.Errorf("设备应该健康, 状态: %s", result.Status)
	}
	if result.LatencyMs <= 0 {
		t.Error("延迟应大于 0")
	}

	// 检查不存在的设备
	result = m.HealthCheck("non-existent-id", "")
	if result.Healthy {
		t.Error("不存在的设备应该不健康")
	}
}

func TestAPIHandlers(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	r := setupTestRouter(t, m)

	// 测试 GET /devices
	req, _ := http.NewRequest("GET", "/api/v1/rdmastorage/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("期望 code 为 0, 实际为 %d", resp.Code)
	}

	// 测试 GET /config
	req, _ = http.NewRequest("GET", "/api/v1/rdmastorage/config", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	// 测试 PUT /config
	newConfig := &RDMAConfig{
		ID:                "test",
		Protocol:          ProtocolRoCEv2,
		MTU:               4096,
		CongestionControl: CongestionDCQCN,
		QoS: QoSConfig{
			Enabled:       true,
			Priority:      3,
			ServiceLevel:  0,
			TrafficClass:  0,
			MaxSGLen:      32,
			MaxInlineData: 256,
			Timeout:       20,
			RetryCount:    7,
			RNRRetry:      7,
		},
		NetworkDetection: NetworkDetection{
			Enabled:          true,
			AutoDetectMTU:    true,
			AutoDetectSpeed:  true,
			DetectionTimeout: 30,
		},
		Advanced: AdvancedConfig{
			MaxQueuePairs:     256,
			MaxCQEntries:      65536,
			MaxMRSize:         1073741824,
			UseEventfd:        true,
			NumaAware:         true,
			CompletionVector:  0,
			MaxSendWR:         1024,
			MaxRecvWR:         1024,
			MaxRDMAReadAtomic: 16,
		},
	}

	configJSON, _ := json.Marshal(newConfig)
	req, _ = http.NewRequest("PUT", "/api/v1/rdmastorage/config", bytes.NewBuffer(configJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	// 测试 POST /targets
	devices := m.GetDevices()
	if len(devices) == 0 {
		t.Fatal("未发现设备")
	}

	targetReq := &StorageTarget{
		Name:       "api-test-target",
		Type:       TargetTypeISCSI,
		DeviceID:   devices[0].ID,
		TargetAddr: "192.168.1.200",
		Port:       3260,
		ISCSISettings: &ISCSISettings{
			TargetIQN:     "iqn.2024-01.com.example:api.target",
			InitiatorIQN:  "iqn.2024-01.com.example:api.initiator",
			HeaderDigest:  true,
			DataDigest:    false,
			ImmediateData: true,
			MaxRecvSegLen: 262144,
			MaxBurstLen:   1048576,
			FirstBurstLen: 262144,
			RDMAEnabled:   true,
		},
	}

	targetJSON, _ := json.Marshal(targetReq)
	req, _ = http.NewRequest("POST", "/api/v1/rdmastorage/targets", bytes.NewBuffer(targetJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusCreated, w.Code)
	}

	// 测试 GET /targets
	req, _ = http.NewRequest("GET", "/api/v1/rdmastorage/targets", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	// 测试 GET /tuning
	req, _ = http.NewRequest("GET", "/api/v1/rdmastorage/tuning", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	// 测试 POST /tuning/apply
	applyReq := struct {
		ProfileID string `json:"profile_id"`
	}{
		ProfileID: "profile-balanced",
	}

	applyJSON, _ := json.Marshal(applyReq)
	req, _ = http.NewRequest("POST", "/api/v1/rdmastorage/tuning/apply", bytes.NewBuffer(applyJSON))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	// 测试 GET /health
	req, _ = http.NewRequest("GET", "/api/v1/rdmastorage/health?device_id="+devices[0].ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusOK, w.Code)
	}

	// 测试 GET /health 缺少参数
	req, _ = http.NewRequest("GET", "/api/v1/rdmastorage/health", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 %d, 实际为 %d", http.StatusBadRequest, w.Code)
	}
}

func TestTypeValidation(t *testing.T) {
	// 测试协议验证
	if !IsValidProtocol(ProtocolRoCEv2) {
		t.Error("RoCEv2 应该是有效协议")
	}
	if !IsValidProtocol(ProtocolIWARP) {
		t.Error("iWARP 应该是有效协议")
	}
	if IsValidProtocol("invalid") {
		t.Error("invalid 不应该是有效协议")
	}

	// 测试目标类型验证
	if !IsValidTargetType(TargetTypeISCSI) {
		t.Error("iSCSI 应该是有效目标类型")
	}
	if !IsValidTargetType(TargetTypeNFS) {
		t.Error("NFS 应该是有效目标类型")
	}
	if IsValidTargetType("invalid") {
		t.Error("invalid 不应该是有效目标类型")
	}

	// 测试拥塞控制算法验证
	if !IsValidCongestionAlgorithm(CongestionDCQCN) {
		t.Error("DCQCN 应该是有效拥塞控制算法")
	}
	if !IsValidCongestionAlgorithm(CongestionDCTCP) {
		t.Error("DCTCP 应该是有效拥塞控制算法")
	}
	if IsValidCongestionAlgorithm("invalid") {
		t.Error("invalid 不应该是有效拥塞控制算法")
	}

	// 测试调优预设类型验证
	if !IsValidProfileType(ProfileLowLatency) {
		t.Error("低延迟应该是有效预设类型")
	}
	if !IsValidProfileType(ProfileHighThroughput) {
		t.Error("高吞吐应该是有效预设类型")
	}
	if !IsValidProfileType(ProfileBalanced) {
		t.Error("平衡应该是有效预设类型")
	}
	if IsValidProfileType("invalid") {
		t.Error("invalid 不应该是有效预设类型")
	}
}

func TestDefaultConfigurations(t *testing.T) {
	// 测试默认 RDMA 配置
	config := DefaultRDMAConfig()
	if config == nil {
		t.Fatal("默认配置不应为空")
	}
	if config.Protocol != ProtocolRoCEv2 {
		t.Errorf("期望协议为 %s, 实际为 %s", ProtocolRoCEv2, config.Protocol)
	}
	if config.MTU != 4096 {
		t.Errorf("期望 MTU 为 4096, 实际为 %d", config.MTU)
	}
	if config.CongestionControl != CongestionDCQCN {
		t.Errorf("期望拥塞控制为 %s, 实际为 %s", CongestionDCQCN, config.CongestionControl)
	}

	// 测试默认调优预设
	profiles := DefaultTuningProfiles()
	if len(profiles) == 0 {
		t.Fatal("默认预设不应为空")
	}

	// 查找平衡模式
	var balancedProfile *TuningProfile
	for i, p := range profiles {
		if p.Type == ProfileBalanced {
			balancedProfile = &profiles[i]
			break
		}
	}
	if balancedProfile == nil {
		t.Fatal("未找到平衡模式预设")
	}
	if !balancedProfile.IsDefault {
		t.Error("平衡模式应该是默认预设")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := setupTestManager(t)
	defer m.Stop()

	// 并发读取设备
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			devices := m.GetDevices()
			if len(devices) == 0 {
				t.Error("并发读取设备失败")
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 并发读写配置
	for i := 0; i < 5; i++ {
		go func() {
			config := m.GetConfig()
			if config == nil {
				t.Error("并发读取配置失败")
			}
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}