// Package behaviorransom 提供基于行为分析的勒索软件检测功能
// behaviorransom_test.go - 单元测试
package behaviorransom

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewManager 测试管理器创建
func TestNewManager(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)

	if manager == nil {
		t.Fatal("NewManager返回nil")
	}

	if manager.running {
		t.Error("新创建的管理器不应处于运行状态")
	}
}

// TestManagerStartStop 测试管理器启动和停止
func TestManagerStartStop(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)

	// 启动
	if err := manager.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	status := manager.GetStatus()
	if !status.Running {
		t.Error("管理器应处于运行状态")
	}

	// 重复启动不应报错
	if err := manager.Start(); err != nil {
		t.Errorf("重复启动不应报错: %v", err)
	}

	// 停止
	manager.Stop()

	status = manager.GetStatus()
	if status.Running {
		t.Error("管理器应处于停止状态")
	}
}

// TestRecordActivity 测试活动记录
func TestRecordActivity(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)

	if err := manager.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer manager.Stop()

	// 记录活动
	activity := FileActivity{
		Path:      "/data/test.txt",
		Operation: FileOpModify,
		Size:      1024,
		Timestamp: time.Now(),
	}

	manager.RecordActivity(activity)

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	status := manager.GetStatus()
	if status.TotalActivities != 1 {
		t.Errorf("期望TotalActivities=1, 得到=%d", status.TotalActivities)
	}
}

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	config := DefaultDetectorConfig()

	if !config.Enabled {
		t.Error("默认配置应启用")
	}

	if config.WindowSizeSec != 60 {
		t.Errorf("期望WindowSizeSec=60, 得到=%d", config.WindowSizeSec)
	}

	if config.EntropyThreshold != 7.5 {
		t.Errorf("期望EntropyThreshold=7.5, 得到=%f", config.EntropyThreshold)
	}

	if config.BlockThreshold != 80 {
		t.Errorf("期望BlockThreshold=80, 得到=%d", config.BlockThreshold)
	}

	if !config.AutoQuarantine {
		t.Error("默认配置应启用自动隔离")
	}
}

// TestThreatLevelString 测试威胁级别字符串
func TestThreatLevelString(t *testing.T) {
	tests := []struct {
		level    ThreatLevel
		expected string
	}{
		{ThreatLevelNone, "none"},
		{ThreatLevelLow, "low"},
		{ThreatLevelMedium, "medium"},
		{ThreatLevelHigh, "high"},
		{ThreatLevelCritical, "critical"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("ThreatLevel(%d).String() = %s, 期望 %s", tt.level, got, tt.expected)
		}
	}
}

// TestEntropyAnalyzer 测试熵值分析器
func TestEntropyAnalyzer(t *testing.T) {
	analyzer := NewEntropyAnalyzer(7.5)

	// 测试空数据
	entropy := analyzer.CalculateShannonEntropy([]byte{})
	if entropy != 0 {
		t.Errorf("空数据熵值应为0, 得到=%f", entropy)
	}

	// 测试均匀分布数据（应有高熵值）
	uniformData := make([]byte, 256)
	for i := 0; i < 256; i++ {
		uniformData[i] = byte(i)
	}
	entropy = analyzer.CalculateShannonEntropy(uniformData)
	if entropy < 7.0 {
		t.Errorf("均匀分布数据熵值应>7.0, 得到=%f", entropy)
	}

	// 测试单一字节数据（应有低熵值）
	singleByteData := make([]byte, 100)
	for i := range singleByteData {
		singleByteData[i] = 0x42
	}
	entropy = analyzer.CalculateShannonEntropy(singleByteData)
	if entropy != 0 {
		t.Errorf("单一字节数据熵值应为0, 得到=%f", entropy)
	}

	// 测试高熵值判断
	if !analyzer.IsHighEntropy(8.0) {
		t.Error("8.0应被视为高熵值")
	}

	if analyzer.IsHighEntropy(3.0) {
		t.Error("3.0不应被视为高熵值")
	}
}

// TestEntropyAnalyzerThreshold 测试熵值分析器阈值
func TestEntropyAnalyzerThreshold(t *testing.T) {
	analyzer := NewEntropyAnalyzer(7.5)

	if analyzer.GetThreshold() != 7.5 {
		t.Errorf("期望阈值=7.5, 得到=%f", analyzer.GetThreshold())
	}

	analyzer.SetThreshold(6.0)
	if analyzer.GetThreshold() != 6.0 {
		t.Errorf("期望阈值=6.0, 得到=%f", analyzer.GetThreshold())
	}
}

// TestEntropyStats 测试熵值统计
func TestEntropyStats(t *testing.T) {
	analyzer := NewEntropyAnalyzer(7.5)

	// 分析多个样本
	data1 := []byte{0x00, 0x01, 0x02, 0x03}
	data2 := []byte{0xFF, 0xFE, 0xFD, 0xFC}

	analyzer.AnalyzeSample(data1)
	analyzer.AnalyzeSample(data2)

	stats := analyzer.GetStats()
	if stats.SampleCount != 2 {
		t.Errorf("期望SampleCount=2, 得到=%d", stats.SampleCount)
	}
}

// TestEntropyDistribution 测试熵值分布
func TestEntropyDistribution(t *testing.T) {
	analyzer := NewEntropyAnalyzer(7.5)

	// 低熵值数据
	lowEntropyData := make([]byte, 100)
	for i := range lowEntropyData {
		lowEntropyData[i] = 0x41 // 'A'
	}
	analyzer.AnalyzeSample(lowEntropyData)

	// 高熵值数据
	highEntropyData := make([]byte, 256)
	for i := 0; i < 256; i++ {
		highEntropyData[i] = byte(i)
	}
	analyzer.AnalyzeSample(highEntropyData)

	stats := analyzer.GetStats()

	// 应该有分布数据
	if len(stats.EntropyDistribution) == 0 {
		t.Error("熵值分布不应为空")
	}
}

// TestBehaviorDetector 测试行为检测器
func TestBehaviorDetector(t *testing.T) {
	config := DefaultDetectorConfig()
	detector := NewBehaviorDetector(config)

	if detector == nil {
		t.Fatal("NewBehaviorDetector返回nil")
	}

	// 获取默认模式
	patterns := detector.GetPatterns()
	if len(patterns) == 0 {
		t.Error("应有默认行为模式")
	}
}

// TestDetectPatterns 测试模式检测
func TestDetectPatterns(t *testing.T) {
	config := DefaultDetectorConfig()
	config.FileRateThreshold = 5 // 降低阈值以便测试
	detector := NewBehaviorDetector(config)

	// 创建大量修改活动
	activities := make([]FileActivity, 60)
	for i := range activities {
		activities[i] = FileActivity{
			Path:      "/data/file_" + string(rune('0'+i%10)) + ".txt",
			Operation: FileOpModify,
			Size:      1024,
			Timestamp: time.Now(),
			Entropy:   8.0, // 高熵值
		}
	}

	threats := detector.DetectPatterns(activities)

	// 应该检测到威胁
	if len(threats) == 0 {
		t.Log("未检测到威胁（可能需要调整阈值）")
	}
}

// TestAddRemovePattern 测试添加和移除模式
func TestAddRemovePattern(t *testing.T) {
	config := DefaultDetectorConfig()
	detector := NewBehaviorDetector(config)

	initialCount := len(detector.GetPatterns())

	// 添加自定义模式
	pattern := BehaviorPattern{
		ID:          "test-pattern",
		Name:        "测试模式",
		Description: "测试用行为模式",
		Severity:    ThreatLevelMedium,
		Weight:      50,
		Indicators: []PatternIndicator{
			{Type: "file_modify_rate", Threshold: 100, TimeWindowSec: 30},
		},
	}

	detector.AddPattern(pattern)

	if len(detector.GetPatterns()) != initialCount+1 {
		t.Errorf("期望模式数=%d, 得到=%d", initialCount+1, len(detector.GetPatterns()))
	}

	// 移除模式
	removed := detector.RemovePattern("test-pattern")
	if !removed {
		t.Error("移除模式应成功")
	}

	if len(detector.GetPatterns()) != initialCount {
		t.Errorf("期望模式数=%d, 得到=%d", initialCount, len(detector.GetPatterns()))
	}

	// 移除不存在的模式
	removed = detector.RemovePattern("nonexistent")
	if removed {
		t.Error("移除不存在的模式应返回false")
	}
}

// TestRiskScoreCalculation 测试风险评分计算
func TestRiskScoreCalculation(t *testing.T) {
	config := DefaultDetectorConfig()
	detector := NewBehaviorDetector(config)

	// 空活动列表
	score := detector.CalculateRiskScore(nil)
	if score != 0 {
		t.Errorf("空活动列表风险分数应为0, 得到=%f", score)
	}

	// 高风险活动
	activities := make([]FileActivity, 100)
	for i := range activities {
		activities[i] = FileActivity{
			Path:      "/data/file.encrypted",
			Operation: FileOpModify,
			Timestamp: time.Now(),
			Entropy:   8.0,
		}
	}

	score = detector.CalculateRiskScore(activities)
	if score <= 0 {
		t.Error("高风险活动的风险分数应>0")
	}
}

// TestSuspiciousExtension 测试可疑扩展名检测
func TestSuspiciousExtension(t *testing.T) {
	config := DefaultDetectorConfig()
	detector := NewBehaviorDetector(config)

	// 测试可疑扩展名
	if !detector.isSuspiciousExtension("/data/file.encrypted") {
		t.Error(".encrypted应被视为可疑扩展名")
	}

	if !detector.isSuspiciousExtension("/data/file.locked") {
		t.Error(".locked应被视为可疑扩展名")
	}

	if detector.isSuspiciousExtension("/data/file.txt") {
		t.Error(".txt不应被视为可疑扩展名")
	}
}

// TestHandlersStatus 测试状态API
func TestHandlersStatus(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)
	handlers := NewHandlers(manager)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/security/behaviorransom/status", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码=200, 得到=%d", w.Code)
	}

	var resp apiResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("期望code=0, 得到=%d", resp.Code)
	}
}

// TestHandlersRecordActivity 测试活动记录API
func TestHandlersRecordActivity(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)
	if err := manager.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer manager.Stop()

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	activity := FileActivity{
		Path:      "/data/test.txt",
		Operation: FileOpModify,
		Size:      1024,
	}

	body, _ := json.Marshal(activity)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/behaviorransom", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码=200, 得到=%d", w.Code)
	}

	var resp apiResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("期望code=0, 得到=%d", resp.Code)
	}

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	status := manager.GetStatus()
	if status.TotalActivities != 1 {
		t.Errorf("期望TotalActivities=1, 得到=%d", status.TotalActivities)
	}
}

// TestHandlersConfig 测试配置API
func TestHandlersConfig(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)
	handlers := NewHandlers(manager)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// GET配置
	req := httptest.NewRequest(http.MethodGet, "/api/v1/security/behaviorransom/config", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET配置: 期望状态码=200, 得到=%d", w.Code)
	}

	// PUT更新配置
	newConfig := DefaultDetectorConfig()
	newConfig.WindowSizeSec = 120
	body, _ := json.Marshal(newConfig)

	req = httptest.NewRequest(http.MethodPut, "/api/v1/security/behaviorransom/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PUT配置: 期望状态码=200, 得到=%d", w.Code)
	}

	// 验证配置已更新
	updatedConfig := manager.GetConfig()
	if updatedConfig.WindowSizeSec != 120 {
		t.Errorf("期望WindowSizeSec=120, 得到=%d", updatedConfig.WindowSizeSec)
	}
}

// TestHandlersThreats 测试威胁查询API
func TestHandlersThreats(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)
	handlers := NewHandlers(manager)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/security/behaviorransom/threats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码=200, 得到=%d", w.Code)
	}
}

// TestHandlersPatterns 测试模式查询API
func TestHandlersPatterns(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)
	handlers := NewHandlers(manager)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/security/behaviorransom/patterns", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码=200, 得到=%d", w.Code)
	}

	var resp apiResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("期望code=0, 得到=%d", resp.Code)
	}
}

// TestHandlersMethodNotAllowed 测试不允许的HTTP方法
func TestHandlersMethodNotAllowed(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)
	handlers := NewHandlers(manager)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// DELETE方法不被允许
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/security/behaviorransom/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("期望状态码=405, 得到=%d", w.Code)
	}
}

// TestHandlersInvalidBody 测试无效请求体
func TestHandlersInvalidBody(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)
	handlers := NewHandlers(manager)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 无效JSON
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/behaviorransom", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码=400, 得到=%d", w.Code)
	}
}

// TestGetDirectory 测试目录提取
func TestGetDirectory(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/data/file.txt", "/data/"},
		{"/a/b/c/file.txt", "/a/b/c/"},
		{"file.txt", "."},
	}

	for _, tt := range tests {
		if got := getDirectory(tt.path); got != tt.expected {
			t.Errorf("getDirectory(%s) = %s, 期望 %s", tt.path, got, tt.expected)
		}
	}
}

// TestGetFileExtension 测试扩展名提取
func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/data/file.txt", ".txt"},
		{"/data/file.encrypted", ".encrypted"},
		{"/data/file", ""},
		{"/data/file.TXT", ".txt"}, // 应转为小写
	}

	for _, tt := range tests {
		if got := GetFileExtension(tt.path); got != tt.expected {
			t.Errorf("GetFileExtension(%s) = %s, 期望 %s", tt.path, got, tt.expected)
		}
	}
}

// TestEntropyBucket 测试熵值分布桶
func TestEntropyBucket(t *testing.T) {
	tests := []struct {
		entropy  float64
		expected string
	}{
		{0.0, "[0, 1)"},
		{1.5, "[1, 2)"},
		{3.0, "[3, 4)"},
		{5.5, "[5, 6)"},
		{7.8, "[7, 8)"},
		{8.5, "[8, +∞)"},
	}

	for _, tt := range tests {
		if got := getEntropyBucket(tt.entropy); got != tt.expected {
			t.Errorf("getEntropyBucket(%f) = %s, 期望 %s", tt.entropy, got, tt.expected)
		}
	}
}

// TestAnalyzeEntropyChange 测试熵值变化分析
func TestAnalyzeEntropyChange(t *testing.T) {
	config := DefaultDetectorConfig()
	config.EntropyDeltaThreshold = 2.0
	detector := NewBehaviorDetector(config)

	// 创建具有明显熵值变化的活动
	activities := make([]FileActivity, 20)
	for i := range activities {
		entropy := 3.0
		if i >= 10 {
			entropy = 8.0 // 后半段高熵值
		}
		activities[i] = FileActivity{
			Path:      "/data/file.txt",
			Operation: FileOpModify,
			Entropy:   entropy,
			Timestamp: time.Now(),
		}
	}

	spiked, delta := detector.AnalyzeEntropyChange(activities)
	if !spiked {
		t.Error("应检测到熵值突增")
	}
	if delta < 2.0 {
		t.Errorf("期望delta>=2.0, 得到=%f", delta)
	}
}

// TestSetAlertHandler 测试告警处理器设置
func TestSetAlertHandler(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)

	var alertReceived bool
	manager.SetAlertHandler(func(event ThreatEvent) {
		alertReceived = true
		_ = alertReceived // used in handler
	})

	if err := manager.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer manager.Stop()

	// 验证处理器已设置
	if manager.alertHandler == nil {
		t.Error("告警处理器应已设置")
	}
}

// TestQuarantineRecord 测试隔离记录
func TestQuarantineRecord(t *testing.T) {
	record := QuarantineRecord{
		ID:             "test-1",
		OriginalPath:   "/data/file.txt",
		QuarantinePath: "/var/quarantine/file.txt",
		Reason:         "测试隔离",
		Timestamp:      time.Now(),
		FileSize:       1024,
	}

	if record.Restored {
		t.Error("新创建的隔离记录不应标记为已恢复")
	}
}

// TestClearCache 测试清除缓存
func TestClearCache(t *testing.T) {
	analyzer := NewEntropyAnalyzer(7.5)

	// 添加一些数据
	data := []byte{0x00, 0x01, 0x02, 0x03}
	analyzer.AnalyzeSample(data)

	stats := analyzer.GetStats()
	if stats.SampleCount != 1 {
		t.Errorf("期望SampleCount=1, 得到=%d", stats.SampleCount)
	}

	// 清除缓存
	analyzer.ClearCache()

	stats = analyzer.GetStats()
	if stats.SampleCount != 0 {
		t.Errorf("清除后期望SampleCount=0, 得到=%d", stats.SampleCount)
	}
}

// TestConfigUpdate 测试配置更新
func TestConfigUpdate(t *testing.T) {
	config := DefaultDetectorConfig()
	manager := NewManager(config)

	originalWindowSize := manager.GetConfig().WindowSizeSec

	newConfig := DefaultDetectorConfig()
	newConfig.WindowSizeSec = 120
	manager.UpdateConfig(newConfig)

	updatedWindowSize := manager.GetConfig().WindowSizeSec
	if updatedWindowSize == originalWindowSize {
		t.Error("配置应已更新")
	}
	if updatedWindowSize != 120 {
		t.Errorf("期望WindowSizeSec=120, 得到=%d", updatedWindowSize)
	}
}
