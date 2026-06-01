// Package dnsmanager 提供 DNS 管理服务器单元测试
package dnsmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupTestManager() *Manager {
	return NewManager()
}

func setupTestHandlers() (*Handlers, *Manager) {
	mgr := NewManager()
	h := NewHandlers(mgr)
	return h, mgr
}

// ========== Manager 测试 ==========

// TestNewManager 测试创建管理器
func TestNewManager(t *testing.T) {
	mgr := setupTestManager()

	// 应该有默认的上游服务器
	assert.Equal(t, 3, len(mgr.upstreams))

	// 应该有默认的拦截规则
	assert.True(t, len(mgr.rules) >= 10)
}

// TestAddRecord 测试添加记录
func TestAddRecord(t *testing.T) {
	mgr := setupTestManager()

	record, err := mgr.AddRecord("example.com", DNSRecord{
		Name:  "www.example.com",
		Type:  RecordTypeA,
		Value: "1.2.3.4",
		TTL:   300,
	})

	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, "www.example.com", record.Name)
	assert.Equal(t, RecordTypeA, record.Type)
	assert.Equal(t, "1.2.3.4", record.Value)
	assert.True(t, record.Enabled)
}

// TestAddRecordValidation 测试记录验证
func TestAddRecordValidation(t *testing.T) {
	mgr := setupTestManager()

	// 空名称
	_, err := mgr.AddRecord("", DNSRecord{
		Name:  "",
		Type:  RecordTypeA,
		Value: "1.2.3.4",
	})
	assert.Error(t, err)

	// 空值
	_, err = mgr.AddRecord("", DNSRecord{
		Name:  "test.com",
		Type:  RecordTypeA,
		Value: "",
	})
	assert.Error(t, err)

	// 无效类型
	_, err = mgr.AddRecord("", DNSRecord{
		Name:  "test.com",
		Type:  "INVALID",
		Value: "1.2.3.4",
	})
	assert.Error(t, err)
}

// TestUpdateRecord 测试更新记录
func TestUpdateRecord(t *testing.T) {
	mgr := setupTestManager()

	record, _ := mgr.AddRecord("", DNSRecord{
		Name:  "test.com",
		Type:  RecordTypeA,
		Value: "1.1.1.1",
	})

	newValue := "2.2.2.2"
	updated, err := mgr.UpdateRecord(record.ID, UpdateRecordRequest{
		Value: &newValue,
	})

	assert.NoError(t, err)
	assert.Equal(t, "2.2.2.2", updated.Value)
}

// TestDeleteRecord 测试删除记录
func TestDeleteRecord(t *testing.T) {
	mgr := setupTestManager()

	record, _ := mgr.AddRecord("", DNSRecord{
		Name:  "test.com",
		Type:  RecordTypeA,
		Value: "1.1.1.1",
	})

	err := mgr.DeleteRecord(record.ID)
	assert.NoError(t, err)

	// 验证已删除
	_, err = mgr.UpdateRecord(record.ID, UpdateRecordRequest{})
	assert.Error(t, err)
}

// TestListRecords 测试列出记录
func TestListRecords(t *testing.T) {
	mgr := setupTestManager()

	mgr.AddRecord("", DNSRecord{Name: "a.com", Type: RecordTypeA, Value: "1.1.1.1"})
	mgr.AddRecord("", DNSRecord{Name: "b.com", Type: RecordTypeA, Value: "2.2.2.2"})

	records, err := mgr.ListRecords("")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(records))
}

// TestAddRule 测试添加规则
func TestAddRule(t *testing.T) {
	mgr := setupTestManager()

	rule, err := mgr.AddRule(DNSRule{
		Pattern:  "ads.example.com",
		Action:   ActionBlock,
		Category: "ads",
	})

	assert.NoError(t, err)
	assert.NotNil(t, rule)
	assert.Equal(t, "ads.example.com", rule.Pattern)
	assert.Equal(t, ActionBlock, rule.Action)
	assert.True(t, rule.Enabled)
}

// TestAddRuleValidation 测试规则验证
func TestAddRuleValidation(t *testing.T) {
	mgr := setupTestManager()

	// 空模式
	_, err := mgr.AddRule(DNSRule{
		Pattern: "",
		Action:  ActionBlock,
	})
	assert.Error(t, err)

	// 无效动作
	_, err = mgr.AddRule(DNSRule{
		Pattern: "test.com",
		Action:  "invalid",
	})
	assert.Error(t, err)

	// 重定向无目标
	_, err = mgr.AddRule(DNSRule{
		Pattern: "test.com",
		Action:  ActionRedirect,
	})
	assert.Error(t, err)
}

// TestUpdateRule 测试更新规则
func TestUpdateRule(t *testing.T) {
	mgr := setupTestManager()

	rule, _ := mgr.AddRule(DNSRule{
		Pattern: "test.com",
		Action:  ActionBlock,
	})

	newPattern := "newtest.com"
	updated, err := mgr.UpdateRule(rule.ID, UpdateRuleRequest{
		Pattern: &newPattern,
	})

	assert.NoError(t, err)
	assert.Equal(t, "newtest.com", updated.Pattern)
}

// TestDeleteRule 测试删除规则
func TestDeleteRule(t *testing.T) {
	mgr := setupTestManager()

	rule, _ := mgr.AddRule(DNSRule{
		Pattern: "test.com",
		Action:  ActionBlock,
	})

	err := mgr.DeleteRule(rule.ID)
	assert.NoError(t, err)

	rules, _ := mgr.ListRules()
	for _, r := range rules {
		assert.NotEqual(t, rule.ID, r.ID)
	}
}

// TestListRules 测试列出规则
func TestListRules(t *testing.T) {
	mgr := setupTestManager()

	rules, err := mgr.ListRules()
	assert.NoError(t, err)
	assert.True(t, len(rules) >= 10) // 默认规则
}

// TestToggleRule 测试切换规则状态
func TestToggleRule(t *testing.T) {
	mgr := setupTestManager()

	rule, _ := mgr.AddRule(DNSRule{
		Pattern: "test.com",
		Action:  ActionBlock,
	})

	assert.True(t, rule.Enabled)

	err := mgr.ToggleRule(rule.ID)
	assert.NoError(t, err)

	// 重新获取规则验证状态已改变
	rules, _ := mgr.ListRules()
	for _, r := range rules {
		if r.ID == rule.ID {
			assert.False(t, r.Enabled)
		}
	}
}

// TestResolve 测试解析
func TestResolve(t *testing.T) {
	mgr := setupTestManager()

	mgr.AddRecord("", DNSRecord{
		Name:  "test.com",
		Type:  RecordTypeA,
		Value: "1.2.3.4",
	})

	record, err := mgr.Resolve("test.com", "A")
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", record.Value)
}

// TestResolveNotFound 测试解析未找到
func TestResolveNotFound(t *testing.T) {
	mgr := setupTestManager()

	_, err := mgr.Resolve("nonexistent.com", "A")
	assert.Error(t, err)
}

// TestShouldBlock 测试拦截检查
func TestShouldBlock(t *testing.T) {
	mgr := setupTestManager()

	// 默认规则应该拦截 ads.google.com
	blocked, rule, err := mgr.ShouldBlock("ads.google.com")
	assert.NoError(t, err)
	assert.True(t, blocked)
	assert.NotEmpty(t, rule)
}

// TestShouldNotBlock 测试放行检查
func TestShouldNotBlock(t *testing.T) {
	mgr := setupTestManager()

	// 正常域名不应该被拦截
	blocked, _, err := mgr.ShouldBlock("www.google.com")
	assert.NoError(t, err)
	assert.False(t, blocked)
}

// TestShouldBlockWithAllow 测试白名单规则
func TestShouldBlockWithAllow(t *testing.T) {
	mgr := setupTestManager()

	// 添加允许规则
	mgr.AddRule(DNSRule{
		Pattern: "allowed.example.com",
		Action:  ActionAllow,
	})

	blocked, _, err := mgr.ShouldBlock("allowed.example.com")
	assert.NoError(t, err)
	assert.False(t, blocked)
}

// TestLogQuery 测试记录查询
func TestLogQuery(t *testing.T) {
	mgr := setupTestManager()

	err := mgr.LogQuery(DNSQuery{
		Client:  "192.168.1.1",
		Domain:  "test.com",
		Type:    "A",
		Blocked: false,
	})

	assert.NoError(t, err)

	logs, total, _ := mgr.GetQueryLog(10, 0)
	assert.Equal(t, 1, total)
	assert.Equal(t, 1, len(logs))
}

// TestGetStats 测试获取统计
func TestGetStats(t *testing.T) {
	mgr := setupTestManager()

	// 添加一些查询日志
	mgr.LogQuery(DNSQuery{Client: "192.168.1.1", Domain: "test1.com", Type: "A", Blocked: false})
	mgr.LogQuery(DNSQuery{Client: "192.168.1.1", Domain: "ads.example.com", Type: "A", Blocked: true})
	mgr.LogQuery(DNSQuery{Client: "192.168.1.2", Domain: "test2.com", Type: "A", Blocked: false})

	stats, err := mgr.GetStats("")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalQueries)
	assert.Equal(t, int64(1), stats.BlockedQueries)
	assert.Equal(t, int64(2), stats.AllowedQueries)
}

// TestGetStatsWithPeriod 测试按时间段获取统计
func TestGetStatsWithPeriod(t *testing.T) {
	mgr := setupTestManager()

	// 添加旧日志
	oldQuery := DNSQuery{
		Client:    "192.168.1.1",
		Domain:    "old.com",
		Type:      "A",
		Blocked:   false,
		Timestamp: time.Now().Add(-48 * time.Hour),
	}
	mgr.LogQuery(oldQuery)

	// 添加新日志
	mgr.LogQuery(DNSQuery{Client: "192.168.1.1", Domain: "new.com", Type: "A", Blocked: false})

	// 按天统计应该只包含新日志
	stats, _ := mgr.GetStats("day")
	assert.Equal(t, int64(1), stats.TotalQueries)
}

// TestAddUpstream 测试添加上游服务器
func TestAddUpstream(t *testing.T) {
	mgr := setupTestManager()

	server, err := mgr.AddUpstream(UpstreamServer{
		Address:  "114.114.114.114",
		Port:     53,
		Protocol: ProtocolUDP,
	})

	assert.NoError(t, err)
	assert.NotNil(t, server)
	assert.Equal(t, "114.114.114.114", server.Address)
	assert.True(t, server.Enabled)
}

// TestAddUpstreamValidation 测试上游服务器验证
func TestAddUpstreamValidation(t *testing.T) {
	mgr := setupTestManager()

	// 空地址
	_, err := mgr.AddUpstream(UpstreamServer{
		Address:  "",
		Port:     53,
		Protocol: ProtocolUDP,
	})
	assert.Error(t, err)

	// 无效端口
	_, err = mgr.AddUpstream(UpstreamServer{
		Address:  "8.8.8.8",
		Port:     0,
		Protocol: ProtocolUDP,
	})
	assert.Error(t, err)

	// 无效协议
	_, err = mgr.AddUpstream(UpstreamServer{
		Address:  "8.8.8.8",
		Port:     53,
		Protocol: "invalid",
	})
	assert.Error(t, err)
}

// TestRemoveUpstream 测试删除上游服务器
func TestRemoveUpstream(t *testing.T) {
	mgr := setupTestManager()

	server, _ := mgr.AddUpstream(UpstreamServer{
		Address:  "114.114.114.114",
		Port:     53,
		Protocol: ProtocolUDP,
	})

	err := mgr.RemoveUpstream(server.ID)
	assert.NoError(t, err)
}

// TestTestUpstream 测试测试上游服务器
func TestTestUpstream(t *testing.T) {
	mgr := setupTestManager()

	// 获取第一个默认服务器
	var serverID string
	for id := range mgr.upstreams {
		serverID = id
		break
	}

	latency, err := mgr.TestUpstream(serverID)
	assert.NoError(t, err)
	assert.True(t, latency > 0)
}

// TestMatchDomain 测试域名匹配
func TestMatchDomain(t *testing.T) {
	// 精确匹配
	assert.True(t, matchDomain("example.com", "example.com"))

	// 子域名匹配
	assert.True(t, matchDomain("sub.example.com", "example.com"))
	assert.True(t, matchDomain("deep.sub.example.com", "example.com"))

	// 通配符匹配
	assert.True(t, matchDomain("example.com", "*.example.com"))
	assert.True(t, matchDomain("sub.example.com", "*.example.com"))

	// 不匹配
	assert.False(t, matchDomain("other.com", "example.com"))
	assert.False(t, matchDomain("example.org", "example.com"))
}

// TestExportConfig 测试导出配置
func TestExportConfig(t *testing.T) {
	mgr := setupTestManager()

	data, err := mgr.ExportConfig()
	assert.NoError(t, err)
	assert.NotNil(t, data)

	// 验证是有效的 JSON
	var config map[string]interface{}
	err = json.Unmarshal(data, &config)
	assert.NoError(t, err)
	assert.Contains(t, config, "records")
	assert.Contains(t, config, "rules")
	assert.Contains(t, config, "upstreams")
}

// ========== Handlers 测试 ==========

// TestHandlersCreateRecord 测试创建记录 API
func TestHandlersCreateRecord(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	reqBody := CreateRecordRequest{
		Zone:  "example.com",
		Name:  "www.example.com",
		Type:  RecordTypeA,
		Value: "1.2.3.4",
		TTL:   300,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dnsmanager/records", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersListRecords 测试列出记录 API
func TestHandlersListRecords(t *testing.T) {
	h, mgr := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	mgr.AddRecord("", DNSRecord{Name: "test.com", Type: RecordTypeA, Value: "1.1.1.1"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dnsmanager/records", nil)
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersCreateRule 测试创建规则 API
func TestHandlersCreateRule(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	reqBody := CreateRuleRequest{
		Pattern:  "ads.test.com",
		Action:   ActionBlock,
		Category: "ads",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dnsmanager/rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersListRules 测试列出规则 API
func TestHandlersListRules(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dnsmanager/rules", nil)
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersCreateUpstream 测试创建上游服务器 API
func TestHandlersCreateUpstream(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	reqBody := CreateUpstreamRequest{
		Address:  "114.114.114.114",
		Port:     53,
		Protocol: ProtocolUDP,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dnsmanager/upstreams", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersListUpstreams 测试列出上游服务器 API
func TestHandlersListUpstreams(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dnsmanager/upstreams", nil)
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersStats 测试统计 API
func TestHandlersStats(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dnsmanager/stats", nil)
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersQueryLog 测试查询日志 API
func TestHandlersQueryLog(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dnsmanager/querylog", nil)
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersResolve 测试解析 API
func TestHandlersResolve(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	reqBody := ResolveRequest{
		Domain: "www.google.com",
		Type:   "A",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dnsmanager/resolve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestHandlersExport 测试导出 API
func TestHandlersExport(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dnsmanager/export", nil)
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

// TestHandlersMethodNotAllowed 测试方法不允许
func TestHandlersMethodNotAllowed(t *testing.T) {
	h, _ := setupTestHandlers()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/dnsmanager/records", nil)
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	mgr := setupTestManager()

	done := make(chan bool, 10)

	// 并发添加记录
	for i := 0; i < 10; i++ {
		go func(i int) {
			mgr.AddRecord("", DNSRecord{
				Name:  fmt.Sprintf("test%d.com", i),
				Type:  RecordTypeA,
				Value: fmt.Sprintf("1.1.1.%d", i),
			})
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	records, _ := mgr.ListRecords("")
	assert.Equal(t, 10, len(records))
}
