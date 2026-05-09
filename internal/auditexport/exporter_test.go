package auditexport

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// testEntries 返回测试用的审计日志条目
func testEntries() []AuditEntry {
	base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	return []AuditEntry{
		{
			Timestamp: base,
			UserID:    "user001",
			UserName:  "张三",
			Action:    "login",
			Resource:  "/api/v1/login",
			Result:    "success",
			IP:        "192.168.1.100",
			UserAgent: "Mozilla/5.0",
			Details:   "正常登录",
			Severity:  "info",
		},
		{
			Timestamp: base.Add(1 * time.Hour),
			UserID:    "user001",
			UserName:  "张三",
			Action:    "read",
			Resource:  "/shared/docs/report.pdf",
			Result:    "success",
			IP:        "192.168.1.100",
			UserAgent: "Mozilla/5.0",
			Details:   "读取文件",
			Severity:  "info",
		},
		{
			Timestamp: base.Add(2 * time.Hour),
			UserID:    "user002",
			UserName:  "李四",
			Action:    "delete",
			Resource:  "/shared/data/old.csv",
			Result:    "denied",
			IP:        "192.168.1.200",
			UserAgent: "curl/7.88",
			Details:   "权限不足",
			Severity:  "warning",
		},
		{
			Timestamp: base.Add(3 * time.Hour),
			UserID:    "user003",
			UserName:  "王五",
			Action:    "login",
			Resource:  "/api/v1/login",
			Result:    "failed",
			IP:        "10.0.0.50",
			UserAgent: "Python/3.11",
			Details:   "密码错误",
			Severity:  "critical",
		},
		{
			Timestamp: base.Add(3*time.Hour + 10*time.Minute),
			UserID:    "user003",
			UserName:  "王五",
			Action:    "login",
			Resource:  "/api/v1/login",
			Result:    "success",
			IP:        "10.0.0.50",
			UserAgent: "Python/3.11",
			Details:   "重试成功",
			Severity:  "info",
		},
		{
			Timestamp: base.Add(4 * time.Hour),
			UserID:    "admin",
			UserName:  "管理员",
			Action:    "config",
			Resource:  "/system/settings",
			Result:    "success",
			IP:        "192.168.1.1",
			UserAgent: "AdminPanel/1.0",
			Details:   "修改系统配置",
			Severity:  "warning",
		},
		// 非常规时间登录（凌晨 2 点）
		{
			Timestamp: time.Date(2024, 1, 15, 2, 30, 0, 0, time.UTC),
			UserID:    "user001",
			UserName:  "张三",
			Action:    "login",
			Resource:  "/api/v1/login",
			Result:    "success",
			IP:        "192.168.1.55",
			UserAgent: "Mozilla/5.0",
			Details:   "凌晨登录",
			Severity:  "warning",
		},
		// 同一用户不同 IP 登录
		{
			Timestamp: base.Add(5 * time.Hour),
			UserID:    "user001",
			UserName:  "张三",
			Action:    "login",
			Resource:  "/api/v1/login",
			Result:    "success",
			IP:        "172.16.0.1",
			UserAgent: "Safari/17",
			Details:   "从不同网络登录",
			Severity:  "info",
		},
	}
}

func newTestExporter() *Exporter {
	logger, _ := zap.NewDevelopment()
	return NewExporter(logger, testEntries())
}

// TestExportCSV 测试 CSV 导出
func TestExportCSV(t *testing.T) {
	e := newTestExporter()

	data, err := e.ExportCSV(ExportFilter{})
	if err != nil {
		t.Fatalf("ExportCSV 失败: %v", err)
	}

	// 解析 CSV
	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("解析 CSV 失败: %v", err)
	}

	// 验证：表头 + 8 条数据
	if len(records) != 9 {
		t.Fatalf("期望 9 行（含表头），实际 %d 行", len(records))
	}

	// 验证表头
	expectedHeader := []string{"时间戳", "用户ID", "用户名", "操作", "资源", "结果", "IP", "详情", "严重级别"}
	for i, h := range expectedHeader {
		if records[0][i] != h {
			t.Errorf("表头第 %d 列: 期望 %q, 实际 %q", i, h, records[0][i])
		}
	}

	// 验证第一行数据
	if records[1][1] != "user001" {
		t.Errorf("第一行用户ID: 期望 user001, 实际 %s", records[1][1])
	}
	if records[1][3] != "login" {
		t.Errorf("第一行操作: 期望 login, 实际 %s", records[1][3])
	}
}

// TestExportJSON 测试 JSON 导出
func TestExportJSON(t *testing.T) {
	e := newTestExporter()

	data, err := e.ExportJSON(ExportFilter{})
	if err != nil {
		t.Fatalf("ExportJSON 失败: %v", err)
	}

	var entries []AuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}

	if len(entries) != 8 {
		t.Fatalf("期望 8 条记录，实际 %d 条", len(entries))
	}

	// 验证字段完整性
	entry := entries[0]
	if entry.UserID == "" || entry.UserName == "" || entry.Action == "" {
		t.Error("JSON 条目字段不完整")
	}
}

// TestExportFilter 测试过滤条件
func TestExportFilter(t *testing.T) {
	e := newTestExporter()

	// 测试时间范围过滤
	base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	start := base.Add(1 * time.Hour)
	end := base.Add(4 * time.Hour)

	filter := ExportFilter{
		StartTime: &start,
		EndTime:   &end,
	}
	data, err := e.ExportJSON(filter)
	if err != nil {
		t.Fatalf("过滤导出失败: %v", err)
	}

	var entries []AuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}

	// 应该有 5 条记录：read, delete, login(failed), login(success), config
	// start=11:00, end=14:00 是闭区间，包含端点
	if len(entries) != 5 {
		t.Fatalf("时间范围过滤: 期望 5 条，实际 %d 条", len(entries))
	}

	// 测试用户过滤
	filter2 := ExportFilter{
		UserIDs: []string{"user002"},
	}
	data2, _ := e.ExportJSON(filter2)
	var entries2 []AuditEntry
	json.Unmarshal(data2, &entries2)
	if len(entries2) != 1 {
		t.Fatalf("用户过滤: 期望 1 条，实际 %d 条", len(entries2))
	}
	if entries2[0].UserName != "李四" {
		t.Errorf("用户过滤: 期望李四，实际 %s", entries2[0].UserName)
	}

	// 测试操作类型过滤
	filter3 := ExportFilter{
		Actions: []string{"login"},
	}
	data3, _ := e.ExportJSON(filter3)
	var entries3 []AuditEntry
	json.Unmarshal(data3, &entries3)
	if len(entries3) != 4 {
		t.Fatalf("操作过滤: 期望 4 条 login，实际 %d 条", len(entries3))
	}

	// 测试结果过滤
	filter4 := ExportFilter{
		Results: []string{"denied"},
	}
	data4, _ := e.ExportJSON(filter4)
	var entries4 []AuditEntry
	json.Unmarshal(data4, &entries4)
	if len(entries4) != 1 {
		t.Fatalf("结果过滤: 期望 1 条 denied，实际 %d 条", len(entries4))
	}

	// 测试 Limit
	filter5 := ExportFilter{
		Limit: 2,
	}
	data5, _ := e.ExportJSON(filter5)
	var entries5 []AuditEntry
	json.Unmarshal(data5, &entries5)
	if len(entries5) != 2 {
		t.Fatalf("Limit 过滤: 期望 2 条，实际 %d 条", len(entries5))
	}
}

// TestComplianceReport 测试合规报告生成
func TestComplianceReport(t *testing.T) {
	e := newTestExporter()

	base := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	start := base
	end := base.Add(24 * time.Hour)

	report := e.GenerateReport(start, end)

	// 验证基本字段
	if report.TotalEvents != 8 {
		t.Fatalf("TotalEvents: 期望 8, 实际 %d", report.TotalEvents)
	}

	if report.ActionStats == nil {
		t.Fatal("ActionStats 不应为 nil")
	}

	// 验证操作统计 (凌晨 login + 4 条日间 login = 5)
	if report.ActionStats["login"] != 5 {
		t.Errorf("login 统计: 期望 5, 实际 %d", report.ActionStats["login"])
	}
	if report.ActionStats["read"] != 1 {
		t.Errorf("read 统计: 期望 1, 实际 %d", report.ActionStats["read"])
	}

	// 验证结果统计 (4 条日间 success + 1 凌晨 success + 1 config success = 6)
	if report.ResultStats["success"] != 6 {
		t.Errorf("success 统计: 期望 6, 实际 %d", report.ResultStats["success"])
	}
	if report.ResultStats["denied"] != 1 {
		t.Errorf("denied 统计: 期望 1, 实际 %d", report.ResultStats["denied"])
	}
	if report.ResultStats["failed"] != 1 {
		t.Errorf("failed 统计: 期望 1, 实际 %d", report.ResultStats["failed"])
	}

	// 验证安全事件（denied + failed + critical）
	if len(report.SecurityEvents) < 2 {
		t.Errorf("SecurityEvents: 期望至少 2 条, 实际 %d", len(report.SecurityEvents))
	}

	// 验证 TopUsers
	if len(report.TopUsers) == 0 {
		t.Error("TopUsers 不应为空")
	}
	// user001 有 5 条记录，应该排第一
	if report.TopUsers[0].UserID != "user001" {
		t.Errorf("TopUsers[0]: 期望 user001, 实际 %s", report.TopUsers[0].UserID)
	}
	if report.TopUsers[0].ActionCount != 5 {
		t.Errorf("user001 ActionCount: 期望 5, 实际 %d", report.TopUsers[0].ActionCount)
	}
}

// TestDetectAnomalies 测试异常登录检测
func TestDetectAnomalies(t *testing.T) {
	e := newTestExporter()

	anomalies := e.DetectAnomalies(e.entries)

	if len(anomalies) == 0 {
		t.Fatal("应检测到异常登录")
	}

	// user001 从 3 个不同 IP 登录，应该被检测
	found := false
	for _, a := range anomalies {
		if a.UserID == "user001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("user001 的异常登录未被检测")
	}

	// 凌晨登录应该被检测
	foundNight := false
	for _, a := range anomalies {
		if a.Timestamp.Hour() >= 0 && a.Timestamp.Hour() < 6 {
			foundNight = true
			break
		}
	}
	if !foundNight {
		t.Error("凌晨登录未被检测")
	}
}

// TestEmptyExport 测试空结果处理
func TestEmptyExport(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	e := NewExporter(logger, []AuditEntry{})

	// 空数据导出
	data, err := e.ExportCSV(ExportFilter{})
	if err != nil {
		t.Fatalf("空数据 CSV 导出失败: %v", err)
	}

	// 至少有表头
	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("解析空 CSV 失败: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("空数据: 期望 1 行（表头），实际 %d 行", len(records))
	}

	// 空数据 JSON 导出
	data2, err := e.ExportJSON(ExportFilter{})
	if err != nil {
		t.Fatalf("空数据 JSON 导出失败: %v", err)
	}
	var entries []AuditEntry
	if err := json.Unmarshal(data2, &entries); err != nil {
		t.Fatalf("解析空 JSON 失败: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("空 JSON: 期望 0 条，实际 %d 条", len(entries))
	}

	// 空数据生成报告
	report := e.GenerateReport(time.Now().Add(-24*time.Hour), time.Now())
	if report.TotalEvents != 0 {
		t.Errorf("空报告 TotalEvents: 期望 0, 实际 %d", report.TotalEvents)
	}
	if report.SecurityEvents != nil && len(report.SecurityEvents) != 0 {
		t.Errorf("空报告 SecurityEvents: 期望 0, 实际 %d", len(report.SecurityEvents))
	}
}

// TestHandler_Export 测试 HTTP 导出接口
func TestHandler_Export(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()
	e := NewExporter(logger, testEntries())
	h := NewHandlers(logger, e)

	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	// 测试 JSON 导出
	body := `{"format":"json","filter":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("导出接口返回 %d, 期望 200", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["code"].(float64) != 0 {
		t.Errorf("响应 code: 期望 0, 实际 %v", resp["code"])
	}

	// 测试 CSV 导出
	body2 := `{"format":"csv","filter":{}}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/audit/export", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("CSV 导出接口返回 %d, 期望 200", w2.Code)
	}

	// 测试无效请求
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/audit/export", strings.NewReader("invalid"))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusBadRequest {
		t.Errorf("无效请求返回 %d, 期望 400", w3.Code)
	}
}

// TestHandler_Report 测试 HTTP 报告接口
func TestHandler_Report(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()
	e := NewExporter(logger, testEntries())
	h := NewHandlers(logger, e)

	router := gin.New()
	api := router.Group("/api/v1")
	h.RegisterRoutes(api)

	// 测试生成报告
	body := `{
		"start_time": "2024-01-15T00:00:00Z",
		"end_time": "2024-01-16T00:00:00Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/report", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("报告接口返回 %d, 期望 200", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["code"].(float64) != 0 {
		t.Errorf("响应 code: 期望 0, 实际 %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if data["total_events"].(float64) != 8 {
		t.Errorf("报告 total_events: 期望 8, 实际 %v", data["total_events"])
	}

	// 测试默认时间范围（无参数）
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/audit/report", strings.NewReader("{}"))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("默认时间范围报告返回 %d, 期望 200", w2.Code)
	}
}
