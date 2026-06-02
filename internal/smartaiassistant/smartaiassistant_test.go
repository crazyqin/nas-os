package smartaiassistant

import (
	"fmt"
	"testing"
	"time"
)

// TestNewUnifiedAIAssistant 测试创建统一AI助手
func TestNewUnifiedAIAssistant(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	if assistant == nil {
		t.Fatal("创建UnifiedAIAssistant失败")
	}
	
	if assistant.providers == nil {
		t.Error("providers未初始化")
	}
	
	if assistant.conversations == nil {
		t.Error("conversations未初始化")
	}
	
	if assistant.maxHistory != 100 {
		t.Errorf("maxHistory期望100，实际%d", assistant.maxHistory)
	}
	
	if assistant.defaultProvider != nil {
		t.Error("defaultProvider应为nil")
	}
}

// TestRegisterProvider 测试注册AI后端
func TestRegisterProvider(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	providers := assistant.GetProviders()
	if len(providers) != 1 {
		t.Fatalf("期望1个provider，实际%d", len(providers))
	}
	
	if providers[0].Name() != "test-local" {
		t.Errorf("provider名称期望test-local，实际%s", providers[0].Name())
	}
}

// TestSetDefaultProvider 测试设置默认AI后端
func TestSetDefaultProvider(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local1 := NewLocalProvider("local1", "model1")
	local2 := NewLocalProvider("local2", "model2")
	
	assistant.RegisterProvider(local1)
	assistant.RegisterProvider(local2)
	
	// 第一个注册的应该是默认的
	if assistant.defaultProvider.Name() != "local1" {
		t.Errorf("默认provider期望local1，实际%s", assistant.defaultProvider.Name())
	}
	
	// 设置新的默认provider
	assistant.SetDefaultProvider(local2)
	if assistant.defaultProvider.Name() != "local2" {
		t.Errorf("默认provider期望local2，实际%s", assistant.defaultProvider.Name())
	}
}

// TestUpdateSystemStatus 测试更新系统状态
func TestUpdateSystemStatus(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	status := &SystemStatus{
		CPUUsage:    75.5,
		MemoryUsage: 60.2,
		DiskUsage:   45.0,
		Temperature: 55.0,
		Uptime:      86400,
		NetworkUp:   true,
		ServicesOK:  true,
	}
	
	assistant.UpdateSystemStatus(status)
	
	if assistant.systemStatus == nil {
		t.Fatal("systemStatus未更新")
	}
	
	if assistant.systemStatus.CPUUsage != 75.5 {
		t.Errorf("CPUUsage期望75.5，实际%f", assistant.systemStatus.CPUUsage)
	}
	
	if assistant.systemStatus.Uptime != 86400 {
		t.Errorf("Uptime期望86400，实际%d", assistant.systemStatus.Uptime)
	}
}

// TestUpdateStorageStatus 测试更新存储状态
func TestUpdateStorageStatus(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	status := &StorageStatus{
		TotalSpace:   1073741824000, // 1TB
		UsedSpace:    536870912000,  // 500GB
		FreeSpace:    536870912000,  // 500GB
		RAIDStatus:   "active",
		DiskCount:    4,
		HealthStatus: "healthy",
	}
	
	assistant.UpdateStorageStatus(status)
	
	if assistant.storageStatus == nil {
		t.Fatal("storageStatus未更新")
	}
	
	if assistant.storageStatus.RAIDStatus != "active" {
		t.Errorf("RAIDStatus期望active，实际%s", assistant.storageStatus.RAIDStatus)
	}
	
	if assistant.storageStatus.DiskCount != 4 {
		t.Errorf("DiskCount期望4，实际%d", assistant.storageStatus.DiskCount)
	}
}

// TestQuery 测试查询功能
func TestQuery(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 测试空查询
	_, err := assistant.Query("test-session", "")
	if err == nil {
		t.Error("空查询应该返回错误")
	}
	
	// 测试正常查询
	result, err := assistant.Query("test-session", "查看系统状态")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	
	if result == nil {
		t.Fatal("结果为nil")
	}
	
	if result.Action != ActionStatus {
		t.Errorf("操作类型期望status，实际%s", result.Action)
	}
	
	if result.Response == "" {
		t.Error("响应内容为空")
	}
	
	if result.Timestamp.IsZero() {
		t.Error("时间戳未设置")
	}
}

// TestQueryWithSystemContext 测试带系统上下文的查询
func TestQueryWithSystemContext(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 设置系统状态
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    80.0,
		MemoryUsage: 70.0,
		DiskUsage:   50.0,
		Temperature: 60.0,
		Uptime:      3600,
		NetworkUp:   true,
		ServicesOK:  true,
	})
	
	result, err := assistant.Query("test-session", "查询CPU使用率")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	
	if result.Context == nil {
		t.Fatal("上下文为nil")
	}
	
	if _, exists := result.Context["system_status"]; !exists {
		t.Error("上下文缺少system_status")
	}
}

// TestQueryNoProvider 测试无AI后端的查询
func TestQueryNoProvider(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	_, err := assistant.Query("test-session", "test query")
	if err == nil {
		t.Error("无provider时应该返回错误")
	}
}

// TestDiagnose 测试诊断功能
func TestDiagnose(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 测试空症状
	_, err := assistant.Diagnose("test-session", "")
	if err == nil {
		t.Error("空症状应该返回错误")
	}
	
	// 测试CPU过高诊断
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    95.0,
		MemoryUsage: 50.0,
		DiskUsage:   30.0,
		Temperature: 50.0,
		Uptime:      3600,
		NetworkUp:   true,
		ServicesOK:  true,
	})
	
	diagnosis, err := assistant.Diagnose("test-session", "系统运行缓慢")
	if err != nil {
		t.Fatalf("诊断失败: %v", err)
	}
	
	if diagnosis.IssueType != "cpu" {
		t.Errorf("问题类型期望cpu，实际%s", diagnosis.IssueType)
	}
	
	if diagnosis.Severity != "warning" {
		t.Errorf("严重程度期望warning，实际%s", diagnosis.Severity)
	}
	
	if diagnosis.Description != "CPU使用率过高" {
		t.Errorf("描述期望'CPU使用率过高'，实际'%s'", diagnosis.Description)
	}
	
	if len(diagnosis.Suggestions) == 0 {
		t.Error("建议列表为空")
	}
}

// TestDiagnoseMemory 测试内存诊断
func TestDiagnoseMemory(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    50.0,
		MemoryUsage: 95.0,
		DiskUsage:   30.0,
		Temperature: 50.0,
		Uptime:      3600,
		NetworkUp:   true,
		ServicesOK:  true,
	})
	
	diagnosis := assistant.performDiagnosis("内存不足")
	
	if diagnosis.IssueType != "memory" {
		t.Errorf("问题类型期望memory，实际%s", diagnosis.IssueType)
	}
}

// TestDiagnoseTemperature 测试温度诊断
func TestDiagnoseTemperature(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    50.0,
		MemoryUsage: 50.0,
		DiskUsage:   30.0,
		Temperature: 85.0,
		Uptime:      3600,
		NetworkUp:   true,
		ServicesOK:  true,
	})
	
	diagnosis := assistant.performDiagnosis("温度过高")
	
	if diagnosis.IssueType != "temperature" {
		t.Errorf("问题类型期望temperature，实际%s", diagnosis.IssueType)
	}
	
	if diagnosis.Severity != "error" {
		t.Errorf("严重程度期望error，实际%s", diagnosis.Severity)
	}
}

// TestDiagnoseNetwork 测试网络诊断
func TestDiagnoseNetwork(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    50.0,
		MemoryUsage: 50.0,
		DiskUsage:   30.0,
		Temperature: 50.0,
		Uptime:      3600,
		NetworkUp:   false,
		ServicesOK:  true,
	})
	
	diagnosis := assistant.performDiagnosis("网络断开")
	
	if diagnosis.IssueType != "network" {
		t.Errorf("问题类型期望network，实际%s", diagnosis.IssueType)
	}
}

// TestDiagnoseStorage 测试存储诊断
func TestDiagnoseStorage(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	assistant.UpdateStorageStatus(&StorageStatus{
		TotalSpace:   1073741824000,
		UsedSpace:    966367641600, // 90%
		FreeSpace:    107374182400,
		RAIDStatus:   "degraded",
		DiskCount:    4,
		HealthStatus: "warning",
	})
	
	diagnosis := assistant.performDiagnosis("存储空间不足")
	
	if diagnosis.IssueType != "storage" {
		t.Errorf("问题类型期望storage，实际%s", diagnosis.IssueType)
	}
}

// TestDiagnosePerformance 测试性能诊断
func TestDiagnosePerformance(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    50.0,
		MemoryUsage: 50.0,
		DiskUsage:   30.0,
		Temperature: 50.0,
		Uptime:      3600,
		NetworkUp:   true,
		ServicesOK:  true,
	})
	
	diagnosis := assistant.performDiagnosis("系统很慢")
	
	if diagnosis.IssueType != "performance" {
		t.Errorf("问题类型期望performance，实际%s", diagnosis.IssueType)
	}
}

// TestDiagnoseHardware 测试硬件诊断
func TestDiagnoseHardware(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	diagnosis := assistant.performDiagnosis("硬盘有噪音")
	
	if diagnosis.IssueType != "hardware" {
		t.Errorf("问题类型期望hardware，实际%s", diagnosis.IssueType)
	}
}

// TestDiagnoseUnknown 测试未知诊断
func TestDiagnoseUnknown(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	diagnosis := assistant.performDiagnosis("其他问题")
	
	if diagnosis.IssueType != "unknown" {
		t.Errorf("问题类型期望unknown，实际%s", diagnosis.IssueType)
	}
	
	if diagnosis.Severity != "info" {
		t.Errorf("严重程度期望info，实际%s", diagnosis.Severity)
	}
}

// TestSuggest 测试建议功能
func TestSuggest(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 测试空场景
	_, err := assistant.Suggest("test-session", "")
	if err == nil {
		t.Error("空场景应该返回错误")
	}
	
	// 测试备份建议
	suggestions, err := assistant.Suggest("test-session", "如何备份数据")
	if err != nil {
		t.Fatalf("建议生成失败: %v", err)
	}
	
	if len(suggestions) == 0 {
		t.Fatal("建议列表为空")
	}
	
	found := false
	for _, s := range suggestions {
		if s.Category == "backup" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("未找到备份相关建议")
	}
}

// TestSuggestPerformance 测试性能优化建议
func TestSuggestPerformance(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	suggestions := assistant.generateSuggestions("如何优化性能")
	
	found := false
	for _, s := range suggestions {
		if s.Category == "performance" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("未找到性能优化建议")
	}
}

// TestSuggestSecurity 测试安全建议
func TestSuggestSecurity(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	suggestions := assistant.generateSuggestions("如何加强安全防护")
	
	found := false
	for _, s := range suggestions {
		if s.Category == "security" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("未找到安全建议")
	}
}

// TestSuggestStorage 测试存储建议
func TestSuggestStorage(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	suggestions := assistant.generateSuggestions("存储空间管理")
	
	found := false
	for _, s := range suggestions {
		if s.Category == "storage" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("未找到存储建议")
	}
}

// TestSuggestDefault 测试默认建议
func TestSuggestDefault(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	suggestions := assistant.generateSuggestions("其他场景")
	
	if len(suggestions) == 0 {
		t.Fatal("默认建议列表为空")
	}
	
	if suggestions[0].Category != "maintenance" {
		t.Errorf("默认建议分类期望maintenance，实际%s", suggestions[0].Category)
	}
}

// TestGetStatus 测试获取状态
func TestGetStatus(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	// 无状态时
	status := assistant.GetStatus()
	if status == nil {
		t.Fatal("状态为nil")
	}
	
	if _, exists := status["providers"]; !exists {
		t.Error("状态缺少providers")
	}
	
	// 有状态时
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage: 50.0,
	})
	
	assistant.UpdateStorageStatus(&StorageStatus{
		TotalSpace: 1073741824000,
	})
	
	status = assistant.GetStatus()
	
	if _, exists := status["system"]; !exists {
		t.Error("状态缺少system")
	}
	
	if _, exists := status["storage"]; !exists {
		t.Error("状态缺少storage")
	}
}

// TestConversationHistory 测试对话历史
func TestConversationHistory(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	sessionID := "test-session"
	
	// 初始历史为空
	history := assistant.GetConversationHistory(sessionID)
	if len(history) != 0 {
		t.Errorf("初始历史应为空，实际%d条", len(history))
	}
	
	// 执行查询
	_, err := assistant.Query(sessionID, "测试查询")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	
	// 验证历史记录
	history = assistant.GetConversationHistory(sessionID)
	if len(history) != 2 {
		t.Errorf("历史应有2条记录，实际%d条", len(history))
	}
	
	if history[0].Role != "user" {
		t.Errorf("第一条消息角色期望user，实际%s", history[0].Role)
	}
	
	if history[1].Role != "assistant" {
		t.Errorf("第二条消息角色期望assistant，实际%s", history[1].Role)
	}
}

// TestClearConversationHistory 测试清空对话历史
func TestClearConversationHistory(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	sessionID := "test-session"
	
	// 执行查询创建历史
	_, _ = assistant.Query(sessionID, "测试查询")
	
	// 清空历史
	assistant.ClearConversationHistory(sessionID)
	
	// 验证历史已清空
	history := assistant.GetConversationHistory(sessionID)
	if len(history) != 0 {
		t.Errorf("历史应为空，实际%d条", len(history))
	}
}

// TestConversationHistoryLimit 测试对话历史限制
func TestConversationHistoryLimit(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	assistant.maxHistory = 5 // 设置较小的限制用于测试
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	sessionID := "test-session"
	
	// 添加多条消息
	for i := 0; i < 10; i++ {
		_, _ = assistant.Query(sessionID, "测试查询")
	}
	
	// 验证历史记录数量
	history := assistant.GetConversationHistory(sessionID)
	if len(history) > 5 {
		t.Errorf("历史应限制在5条以内，实际%d条", len(history))
	}
}

// TestExecuteCommand 测试命令执行
func TestExecuteCommand(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 测试状态命令
	result, err := assistant.ExecuteCommand("test-session", "/status")
	if err != nil {
		t.Fatalf("执行/status命令失败: %v", err)
	}
	
	if result.Action != ActionStatus {
		t.Errorf("操作类型期望status，实际%s", result.Action)
	}
	
	// 测试帮助命令
	result, err = assistant.ExecuteCommand("test-session", "/help")
	if err != nil {
		t.Fatalf("执行/help命令失败: %v", err)
	}
	
	if result.Action != ActionHelp {
		t.Errorf("操作类型期望help，实际%s", result.Action)
	}
	
	// 测试自然语言命令
	result, err = assistant.ExecuteCommand("test-session", "查看系统状态")
	if err != nil {
		t.Fatalf("执行自然语言命令失败: %v", err)
	}
	
	if result.Action != ActionStatus {
		t.Errorf("操作类型期望status，实际%s", result.Action)
	}
}

// TestExecuteCommandDiagnose 测试诊断命令
func TestExecuteCommandDiagnose(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	result, err := assistant.ExecuteCommand("test-session", "/diagnose CPU使用率过高")
	if err != nil {
		t.Fatalf("执行/diagnose命令失败: %v", err)
	}
	
	if result.Action != ActionDiagnose {
		t.Errorf("操作类型期望diagnose，实际%s", result.Action)
	}
}

// TestExecuteCommandSuggest 测试建议命令
func TestExecuteCommandSuggest(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	result, err := assistant.ExecuteCommand("test-session", "/suggest 如何备份数据")
	if err != nil {
		t.Fatalf("执行/suggest命令失败: %v", err)
	}
	
	if result.Action != ActionSuggest {
		t.Errorf("操作类型期望suggest，实际%s", result.Action)
	}
}

// TestExecuteCommandExecute 测试执行命令（需要权限）
func TestExecuteCommandExecute(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	_, err := assistant.ExecuteCommand("test-session", "/execute some command")
	if err == nil {
		t.Error("execute命令应该返回权限错误")
	}
}

// TestClassifyQuery 测试查询分类
func TestClassifyQuery(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	tests := []struct {
		query    string
		expected AIAction
	}{
		{"系统故障", ActionDiagnose},
		{"出现错误", ActionDiagnose},
		{"无法访问", ActionDiagnose},
		{"如何优化", ActionSuggest},
		{"推荐方案", ActionSuggest},
		{"查看状态", ActionStatus},
		{"检查系统", ActionStatus},
		{"帮助信息", ActionHelp},
		{"执行命令", ActionExecute},
		{"查询数据", ActionQuery},
		{"这是什么", ActionQuery},
	}
	
	for _, test := range tests {
		result := assistant.classifyQuery(test.query)
		if result != test.expected {
			t.Errorf("查询'%s'分类期望%s，实际%s", test.query, test.expected, result)
		}
	}
}

// TestParseCommand 测试命令解析
func TestParseCommand(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	tests := []struct {
		input         string
		expectedAction AIAction
		expectedContent string
	}{
		{"/query test", ActionQuery, "test"},
		{"/diagnose symptom", ActionDiagnose, "symptom"},
		{"/suggest scenario", ActionSuggest, "scenario"},
		{"/execute cmd", ActionExecute, "cmd"},
		{"/status", ActionStatus, ""},
		{"/help", ActionHelp, ""},
		{"natural language query", ActionQuery, "natural language query"},
	}
	
	for _, test := range tests {
		action, content := assistant.parseCommand(test.input)
		if action != test.expectedAction {
			t.Errorf("命令'%s'操作类型期望%s，实际%s", test.input, test.expectedAction, action)
		}
		if content != test.expectedContent {
			t.Errorf("命令'%s'内容期望'%s'，实际'%s'", test.input, test.expectedContent, content)
		}
	}
}

// TestGetStats 测试获取统计信息
func TestGetStats(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 执行一些查询创建会话
	_, _ = assistant.Query("session1", "查询1")
	_, _ = assistant.Query("session2", "查询2")
	
	stats := assistant.GetStats()
	
	if stats == nil {
		t.Fatal("统计信息为nil")
	}
	
	if stats["total_providers"] != 1 {
		t.Errorf("total_providers期望1，实际%v", stats["total_providers"])
	}
	
	if stats["active_conversations"] != 2 {
		t.Errorf("active_conversations期望2，实际%v", stats["active_conversations"])
	}
	
	if stats["max_history"] != 100 {
		t.Errorf("max_history期望100，实际%v", stats["max_history"])
	}
	
	if stats["total_messages"] != 4 {
		t.Errorf("total_messages期望4，实际%v", stats["total_messages"])
	}
}

// TestLocalProvider 测试本地LLM提供者
func TestLocalProvider(t *testing.T) {
	provider := NewLocalProvider("test-local", "test-model")
	
	if provider.Name() != "test-local" {
		t.Errorf("名称期望test-local，实际%s", provider.Name())
	}
	
	if !provider.IsAvailable() {
		t.Error("provider应该可用")
	}
	
	// 测试处理查询
	response, err := provider.Process("test query", map[string]string{
		"system_status": "CPU:50%",
	})
	
	if err != nil {
		t.Fatalf("处理查询失败: %v", err)
	}
	
	if response == "" {
		t.Error("响应为空")
	}
	
	// 测试不可用状态
	provider.available = false
	
	_, err = provider.Process("test query", nil)
	if err == nil {
		t.Error("不可用时应该返回错误")
	}
}

// TestRemoteProvider 测试远程API提供者
func TestRemoteProvider(t *testing.T) {
	provider := NewRemoteProvider("test-remote", "http://api.test.com", "test-key")
	
	if provider.Name() != "test-remote" {
		t.Errorf("名称期望test-remote，实际%s", provider.Name())
	}
	
	if !provider.IsAvailable() {
		t.Error("provider应该可用")
	}
	
	// 测试处理查询
	response, err := provider.Process("test query", map[string]string{
		"system_status": "CPU:50%",
	})
	
	if err != nil {
		t.Fatalf("处理查询失败: %v", err)
	}
	
	if response == "" {
		t.Error("响应为空")
	}
	
	// 测试不可用状态
	provider.available = false
	
	_, err = provider.Process("test query", nil)
	if err == nil {
		t.Error("不可用时应该返回错误")
	}
}

// TestFormatDiagnosisResponse 测试诊断响应格式化
func TestFormatDiagnosisResponse(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	diagnosis := &DiagnosisResult{
		IssueType:   "cpu",
		Severity:    "warning",
		Description: "CPU使用率过高",
		Suggestions: []string{"建议1", "建议2"},
		RelatedLogs: []string{"log1", "log2"},
		Timestamp:   time.Now(),
	}
	
	response := assistant.formatDiagnosisResponse(diagnosis)
	
	if response == "" {
		t.Error("响应为空")
	}
	
	if !contains(response, "CPU使用率过高") {
		t.Error("响应缺少问题描述")
	}
	
	if !contains(response, "建议1") {
		t.Error("响应缺少建议内容")
	}
}

// TestFormatSuggestionsResponse 测试建议响应格式化
func TestFormatSuggestionsResponse(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	suggestions := []*Suggestion{
		{
			Title:       "测试建议",
			Description: "测试描述",
			Category:    "test",
			Priority:    5,
			Steps:       []string{"步骤1", "步骤2"},
		},
	}
	
	response := assistant.formatSuggestionsResponse(suggestions)
	
	if response == "" {
		t.Error("响应为空")
	}
	
	if !contains(response, "测试建议") {
		t.Error("响应缺少建议标题")
	}
	
	if !contains(response, "步骤1") {
		t.Error("响应缺少操作步骤")
	}
}

// TestFormatStatusResponse 测试状态响应格式化
func TestFormatStatusResponse(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    50.0,
		MemoryUsage: 60.0,
		DiskUsage:   70.0,
		Temperature: 55.0,
		Uptime:      3600,
		NetworkUp:   true,
		ServicesOK:  true,
	})
	
	response := assistant.formatStatusResponse()
	
	if response == "" {
		t.Error("响应为空")
	}
	
	if !contains(response, "系统状态") {
		t.Error("响应缺少系统状态标题")
	}
}

// TestFormatHelpResponse 测试帮助响应格式化
func TestFormatHelpResponse(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	response := assistant.formatHelpResponse()
	
	if response == "" {
		t.Error("响应为空")
	}
	
	if !contains(response, "使用帮助") {
		t.Error("响应缺少帮助标题")
	}
}

// TestGetSystemContext 测试获取系统上下文
func TestGetSystemContext(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	// 无状态时
	context := assistant.getSystemContext()
	if context == nil {
		t.Fatal("上下文为nil")
	}
	
	if len(context) != 0 {
		t.Error("无状态时上下文应为空")
	}
	
	// 有状态时
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    50.0,
		MemoryUsage: 60.0,
		DiskUsage:   70.0,
		Temperature: 55.0,
		Uptime:      3600,
		NetworkUp:   true,
		ServicesOK:  true,
	})
	
	assistant.UpdateStorageStatus(&StorageStatus{
		TotalSpace:   1073741824000,
		UsedSpace:    536870912000,
		FreeSpace:    536870912000,
		RAIDStatus:   "active",
		DiskCount:    4,
		HealthStatus: "healthy",
	})
	
	context = assistant.getSystemContext()
	
	if _, exists := context["system_status"]; !exists {
		t.Error("上下文缺少system_status")
	}
	
	if _, exists := context["storage"]; !exists {
		t.Error("上下文缺少storage")
	}
	
	if _, exists := context["uptime"]; !exists {
		t.Error("上下文缺少uptime")
	}
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 并发查询
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(index int) {
			sessionID := fmt.Sprintf("session-%d", index)
			_, _ = assistant.Query(sessionID, "并发测试")
			done <- true
		}(i)
	}
	
	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// 验证状态
	stats := assistant.GetStats()
	if stats["active_conversations"] != 10 {
		t.Errorf("并发会话数期望10，实际%v", stats["active_conversations"])
	}
}

// TestMultipleSessions 测试多会话
func TestMultipleSessions(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 创建多个会话
	_, _ = assistant.Query("session1", "会话1查询")
	_, _ = assistant.Query("session2", "会话2查询")
	_, _ = assistant.Query("session3", "会话3查询")
	
	// 验证会话隔离
	history1 := assistant.GetConversationHistory("session1")
	history2 := assistant.GetConversationHistory("session2")
	
	if len(history1) != 2 {
		t.Errorf("会话1历史应有2条，实际%d条", len(history1))
	}
	
	if len(history2) != 2 {
		t.Errorf("会话2历史应有2条，实际%d条", len(history2))
	}
	
	// 清空一个会话不影响其他会话
	assistant.ClearConversationHistory("session1")
	
	history1 = assistant.GetConversationHistory("session1")
	history2 = assistant.GetConversationHistory("session2")
	
	if len(history1) != 0 {
		t.Errorf("会话1历史应为空，实际%d条", len(history1))
	}
	
	if len(history2) != 2 {
		t.Errorf("会话2历史应仍有2条，实际%d条", len(history2))
	}
}

// TestQueryClassificationIntegration 测试查询分类集成
func TestQueryClassificationIntegration(t *testing.T) {
	assistant := NewUnifiedAIAssistant()
	
	local := NewLocalProvider("test-local", "test-model")
	assistant.RegisterProvider(local)
	
	// 设置系统状态触发诊断
	assistant.UpdateSystemStatus(&SystemStatus{
		CPUUsage:    95.0,
		MemoryUsage: 50.0,
		DiskUsage:   30.0,
		Temperature: 50.0,
		Uptime:      3600,
		NetworkUp:   true,
		ServicesOK:  true,
	})
	
	// 查询包含诊断关键词
	result, err := assistant.Query("test-session", "系统出现故障")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	
	if result.Action != ActionDiagnose {
		t.Errorf("操作类型期望diagnose，实际%s", result.Action)
	}
	
	if result.Diagnosis == nil {
		t.Error("诊断结果为nil")
	}
}

// contains 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
