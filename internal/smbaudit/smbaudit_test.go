package smbaudit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newTestAuditor() *Auditor {
	return NewAuditor(&AuditConfig{
		MaxEvents:     100,
		RetentionDays: 30,
		LogPath:       "/tmp/test-smb-audit.log",
	})
}

func newTestRouter(a *Auditor) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandlers(a)
	api := r.Group("/api")
	h.RegisterRoutes(api)
	return r
}

// TestNewAuditor 测试审计器创建和默认配置
func TestNewAuditor(t *testing.T) {
	a := NewAuditor(nil)
	defer a.Close()
	cfg := a.GetConfig()
	assert.Equal(t, 10000, cfg.MaxEvents)
	assert.Equal(t, 90, cfg.RetentionDays)
	assert.Equal(t, "/var/log/nas-os/smb-audit.log", cfg.LogPath)
}

// TestLogEvent 测试记录审计事件
func TestLogEvent(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	err := a.LogEvent("admin", "192.168.1.100", "documents", "/share/file.txt", "read", true, "正常读取")
	assert.NoError(t, err)

	events, total := a.GetEvents(10, 0)
	assert.Equal(t, 1, total)
	assert.Len(t, events, 1)
	assert.Equal(t, "admin", events[0].Username)
	assert.Equal(t, "read", events[0].Action)
	assert.True(t, events[0].Success)
	assert.NotEmpty(t, events[0].EventID)
}

// TestFilteredUsers 测试用户过滤
func TestFilteredUsers(t *testing.T) {
	a := NewAuditor(&AuditConfig{
		MaxEvents:    100,
		FilteredUsers: []string{"system"},
	})
	defer a.Close()

	_ = a.LogEvent("system", "127.0.0.1", "tmp", "/file", "read", true, "")
	_ = a.LogEvent("admin", "192.168.1.1", "docs", "/file", "write", true, "")

	events, total := a.GetEvents(10, 0)
	assert.Equal(t, 1, total)
	assert.Equal(t, "admin", events[0].Username)
}

// TestFilteredShares 测试共享过滤
func TestFilteredShares(t *testing.T) {
	a := NewAuditor(&AuditConfig{
		MaxEvents:      100,
		FilteredShares: []string{"internal"},
	})
	defer a.Close()

	_ = a.LogEvent("user1", "10.0.0.1", "internal", "/secret", "read", true, "")
	_ = a.LogEvent("user1", "10.0.0.1", "public", "/readme", "read", true, "")

	_, total := a.GetEvents(10, 0)
	assert.Equal(t, 1, total)
}

// TestGetEventsByUser 测试按用户查询
func TestGetEventsByUser(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	_ = a.LogEvent("alice", "192.168.1.1", "share1", "/a.txt", "read", true, "")
	_ = a.LogEvent("bob", "192.168.1.2", "share1", "/b.txt", "write", true, "")
	_ = a.LogEvent("alice", "192.168.1.1", "share1", "/c.txt", "delete", false, "权限不足")

	events := a.GetEventsByUser("alice", 10)
	assert.Len(t, events, 2)
	for _, e := range events {
		assert.Equal(t, "alice", e.Username)
	}
}

// TestGetEventsByShare 测试按共享名查询
func TestGetEventsByShare(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	_ = a.LogEvent("user", "10.0.0.1", "documents", "/doc1", "read", true, "")
	_ = a.LogEvent("user", "10.0.0.1", "photos", "/img1", "read", true, "")
	_ = a.LogEvent("user", "10.0.0.1", "documents", "/doc2", "write", true, "")

	events := a.GetEventsByShare("documents", 10)
	assert.Len(t, events, 2)
}

// TestGetEventsByAction 测试按操作类型查询
func TestGetEventsByAction(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "")
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f2", "write", true, "")
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f3", "read", true, "")

	events := a.GetEventsByAction("read", 10)
	assert.Len(t, events, 2)
}

// TestGetFailedEvents 测试获取失败事件
func TestGetFailedEvents(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "")
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f2", "write", false, "磁盘已满")
	_ = a.LogEvent("u2", "2.2.2.2", "s2", "/f3", "delete", false, "权限不足")

	failed := a.GetFailedEvents(10)
	assert.Len(t, failed, 2)
	for _, e := range failed {
		assert.False(t, e.Success)
	}
}

// TestGetEventsByTimeRange 测试按时间范围查询
func TestGetEventsByTimeRange(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	now := time.Now()
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "")

	events := a.GetEventsByTimeRange(now.Add(-1*time.Minute), now.Add(1*time.Minute))
	assert.Len(t, events, 1)
}

// TestClearEvents 测试清理旧事件
func TestClearEvents(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	// 先添加几条事件
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "")
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f2", "write", true, "")

	// 清理当前时间之后的事件（应该清理 0 条）
	removed := a.ClearEvents(time.Now().Add(1 * time.Hour))
	assert.Equal(t, 2, removed)

	_, total := a.GetEvents(10, 0)
	assert.Equal(t, 0, total)
}

// TestExportEventsJSON 测试 JSON 导出
func TestExportEventsJSON(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "test")

	now := time.Now()
	data, err := a.ExportEvents(now.Add(-1*time.Hour), now.Add(1*time.Hour), "json")
	assert.NoError(t, err)

	var events []AuditEvent
	err = json.Unmarshal(data, &events)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "u1", events[0].Username)
}

// TestExportEventsCSV 测试 CSV 导出
func TestExportEventsCSV(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "test")

	now := time.Now()
	data, err := a.ExportEvents(now.Add(-1*time.Hour), now.Add(1*time.Hour), "csv")
	assert.NoError(t, err)

	csvStr := string(data)
	assert.Contains(t, csvStr, "event_id")
	assert.Contains(t, csvStr, "u1")
}

// TestGetAuditStats 测试审计统计
func TestGetAuditStats(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	_ = a.LogEvent("alice", "1.1.1.1", "share1", "/f1", "read", true, "")
	_ = a.LogEvent("bob", "2.2.2.2", "share1", "/f2", "write", false, "")
	_ = a.LogEvent("alice", "1.1.1.1", "share2", "/f3", "delete", true, "")

	stats := a.GetAuditStats()
	assert.Equal(t, 3, stats["total_events"])
	assert.Equal(t, 1, stats["failed_events"])

	byUser := stats["by_user"].(map[string]int)
	assert.Equal(t, 2, byUser["alice"])
	assert.Equal(t, 1, byUser["bob"])
}

// TestUpdateConfig 测试配置更新
func TestUpdateConfig(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	newCfg := AuditConfig{
		MaxEvents:     5000,
		RetentionDays: 60,
		FilteredUsers: []string{"guest"},
	}
	a.UpdateConfig(newCfg)

	cfg := a.GetConfig()
	assert.Equal(t, 5000, cfg.MaxEvents)
	assert.Equal(t, 60, cfg.RetentionDays)
	assert.Contains(t, cfg.FilteredUsers, "guest")
}

// TestMaxEvents 测试最大事件数限制
func TestMaxEvents(t *testing.T) {
	a := NewAuditor(&AuditConfig{
		MaxEvents: 5,
	})
	defer a.Close()

	for i := 0; i < 10; i++ {
		_ = a.LogEvent("user", "1.1.1.1", "share", "/file", "read", true, "")
	}

	_, total := a.GetEvents(100, 0)
	assert.Equal(t, 5, total)
}

// TestHandlerListEvents 测试 HTTP 事件列表接口
func TestHandlerListEvents(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "")

	r := newTestRouter(a)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/smb-audit/events?limit=10", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "u1")
}

// TestHandlerFailedEvents 测试 HTTP 失败事件接口
func TestHandlerFailedEvents(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "write", false, "失败")

	r := newTestRouter(a)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/smb-audit/failed", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "失败")
}

// TestHandlerStats 测试 HTTP 统计接口
func TestHandlerStats(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "")

	r := newTestRouter(a)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/smb-audit/stats", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "total_events")
}

// TestHandlerGetConfig 测试 HTTP 获取配置接口
func TestHandlerGetConfig(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	r := newTestRouter(a)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/smb-audit/config", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "max_events")
}

// TestHandlerUpdateConfig 测试 HTTP 更新配置接口
func TestHandlerUpdateConfig(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()

	r := newTestRouter(a)
	w := httptest.NewRecorder()
	body := `{"max_events":5000,"retention_days":60}`
	req, _ := http.NewRequest("PUT", "/api/smb-audit/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "配置已更新")
}

// TestHandlerExportJSON 测试 HTTP JSON 导出接口
func TestHandlerExportJSON(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "test")

	r := newTestRouter(a)
	w := httptest.NewRecorder()
	now := time.Now()
	body := `{"start":"` + now.Add(-1*time.Hour).Format(time.RFC3339) + `","end":"` + now.Add(1*time.Hour).Format(time.RFC3339) + `","format":"json"}`
	req, _ := http.NewRequest("POST", "/api/smb-audit/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "u1")
}

// TestHandlerClearEvents 测试 HTTP 清理事件接口
func TestHandlerClearEvents(t *testing.T) {
	a := newTestAuditor()
	defer a.Close()
	_ = a.LogEvent("u1", "1.1.1.1", "s1", "/f1", "read", true, "")

	r := newTestRouter(a)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/smb-audit/events?days=0", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "清理完成")
}
