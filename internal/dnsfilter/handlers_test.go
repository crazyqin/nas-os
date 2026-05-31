// Package dnsfilter 提供 REST API 处理器测试
package dnsfilter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager()
	h := NewHandlers(mgr)

	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)

	return r, mgr
}

// TestCreateDNSRecord 测试创建 DNS 记录.
func TestCreateDNSRecord(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateDNSRecordRequest{
		Name:  "example.com",
		Type:  RecordTypeA,
		Value: "1.2.3.4",
		TTL:   300,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dns/records", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "example.com", data["name"])
	assert.Equal(t, "A", data["type"])
	assert.Equal(t, "1.2.3.4", data["value"])
	assert.Equal(t, 300, int(data["ttl"].(float64)))
}

// TestListDNSRecords 测试列出 DNS 记录.
func TestListDNSRecords(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建几条记录
	mgr.CreateDNSRecord(CreateDNSRecordRequest{Name: "a.example.com", Type: RecordTypeA, Value: "1.1.1.1"})
	mgr.CreateDNSRecord(CreateDNSRecordRequest{Name: "b.example.com", Type: RecordTypeA, Value: "2.2.2.2"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/dns/records", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 2, int(data["total"].(float64)))
}

// TestUpdateDNSRecord 测试更新 DNS 记录.
func TestUpdateDNSRecord(t *testing.T) {
	r, mgr := setupTestRouter()

	record := mgr.CreateDNSRecord(CreateDNSRecordRequest{
		Name:  "example.com",
		Type:  RecordTypeA,
		Value: "1.2.3.4",
	})

	newValue := "5.6.7.8"
	reqBody := UpdateDNSRecordRequest{Value: &newValue}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/dns/records/"+record.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "5.6.7.8", data["value"])
}

// TestDeleteDNSRecord 测试删除 DNS 记录.
func TestDeleteDNSRecord(t *testing.T) {
	r, mgr := setupTestRouter()

	record := mgr.CreateDNSRecord(CreateDNSRecordRequest{
		Name:  "example.com",
		Type:  RecordTypeA,
		Value: "1.2.3.4",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/dns/records/"+record.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 确认已删除
	_, err := mgr.GetDNSRecord(record.ID)
	assert.Error(t, err)
}

// TestCreateFilterList 测试创建过滤规则列表.
func TestCreateFilterList(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateFilterListRequest{
		Name:        "广告黑名单",
		Description: "常见的广告域名",
		Type:        FilterListBlock,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dns/lists", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "广告黑名单", data["name"])
	assert.Equal(t, "block", data["type"])
}

// TestFilterRules 测试过滤规则管理.
func TestFilterRules(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建列表
	fl := mgr.CreateFilterList(CreateFilterListRequest{
		Name: "测试黑名单",
		Type: FilterListBlock,
	})

	// 添加规则
	reqBody := CreateFilterRuleRequest{
		Pattern: "ads.example.com",
		Action:  ActionBlock,
		ListID:  fl.ID,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dns/rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// 列出规则
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/dns/rules?list_id="+fl.ID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 1, int(resp.Data.(map[string]interface{})["total"].(float64)))
}

// TestUpstreamDNS 测试上游 DNS 管理.
func TestUpstreamDNS(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateUpstreamDNSRequest{
		Name:     "自定义 DNS",
		Address:  "114.114.114.114:53",
		Protocol: "udp",
		Weight:   1,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dns/upstreams", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "自定义 DNS", data["name"])
	assert.Equal(t, "114.114.114.114:53", data["address"])
}

// TestFilterPolicy 测试过滤策略.
func TestFilterPolicy(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateFilterPolicyRequest{
		Name:      "儿童模式",
		ClientIP:  "192.168.1.100",
		StartTime: "08:00",
		EndTime:   "22:00",
		Weekdays:  []int{1, 2, 3, 4, 5},
		Priority:  10,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dns/policies", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "儿童模式", data["name"])
	assert.Equal(t, "192.168.1.100", data["client_ip"])
}

// TestDNSResolve 测试 DNS 解析.
func TestDNSResolve(t *testing.T) {
	_, mgr := setupTestRouter()

	// 添加过滤规则
	mgr.CreateFilterRule(CreateFilterRuleRequest{
		Pattern: "ads.example.com",
		Action:  ActionBlock,
	})

	// 测试被拦截的域名
	logEntry := mgr.ResolveDNS("ads.example.com", "A", "192.168.1.1", "")
	assert.True(t, logEntry.IsFiltered)
	assert.Equal(t, ActionBlock, logEntry.Action)

	// 测试正常域名
	logEntry = mgr.ResolveDNS("www.example.com", "A", "192.168.1.1", "")
	assert.False(t, logEntry.IsFiltered)
	assert.Equal(t, ActionAllow, logEntry.Action)
}

// TestDomainMatching 测试域名匹配.
func TestDomainMatching(t *testing.T) {
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

// TestQueryLogs 测试查询日志.
func TestQueryLogs(t *testing.T) {
	_, mgr := setupTestRouter()

	// 生成一些查询
	mgr.ResolveDNS("test1.example.com", "A", "192.168.1.1", "")
	mgr.ResolveDNS("test2.example.com", "A", "192.168.1.2", "")
	mgr.ResolveDNS("test3.example.com", "AAAA", "192.168.1.1", "")

	// 获取日志
	logs := mgr.GetQueryLogs(QueryLogRequest{Limit: 10})
	assert.Equal(t, 3, len(logs))

	// 按客户端过滤
	logs = mgr.GetQueryLogs(QueryLogRequest{ClientIP: "192.168.1.1", Limit: 10})
	assert.Equal(t, 2, len(logs))
}

// TestQueryStats 测试查询统计.
func TestQueryStats(t *testing.T) {
	_, mgr := setupTestRouter()

	// 添加规则
	mgr.CreateFilterRule(CreateFilterRuleRequest{
		Pattern: "blocked.com",
		Action:  ActionBlock,
	})

	// 生成查询
	mgr.ResolveDNS("allowed.com", "A", "192.168.1.1", "")
	mgr.ResolveDNS("blocked.com", "A", "192.168.1.1", "")
	mgr.ResolveDNS("blocked.com", "A", "192.168.1.2", "")

	stats := mgr.GetStats()
	assert.Equal(t, int64(3), stats.TotalQueries)
	assert.Equal(t, int64(2), stats.BlockedQueries)
	assert.Equal(t, int64(1), stats.AllowedQueries)
	assert.Equal(t, 2, stats.UniqueClients)
}

// TestDNSStatus 测试 DNS 状态.
func TestDNSStatus(t *testing.T) {
	_, mgr := setupTestRouter()

	// 初始状态应该是停止的
	status := mgr.GetStatus()
	assert.False(t, status.Running)

	// 启动
	err := mgr.Start("0.0.0.0", 5353, 5353)
	assert.NoError(t, err)

	status = mgr.GetStatus()
	assert.True(t, status.Running)

	// 重复启动会报错
	err = mgr.Start("0.0.0.0", 5353, 5353)
	assert.Error(t, err)

	// 停止
	err = mgr.Stop()
	assert.NoError(t, err)

	status = mgr.GetStatus()
	assert.False(t, status.Running)
}

// TestTestDNS 测试 DNS 测试 API.
func TestTestDNS(t *testing.T) {
	r, mgr := setupTestRouter()

	// 添加规则
	mgr.CreateFilterRule(CreateFilterRuleRequest{
		Pattern: "ads.example.com",
		Action:  ActionBlock,
	})

	// 测试被拦截域名
	reqBody := TestDNSRequest{
		Domain: "ads.example.com",
		Type:   "A",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dns/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, true, data["is_filtered"])
	assert.Equal(t, "block", data["action"])
}

// TestCache 测试缓存管理.
func TestCache(t *testing.T) {
	r, mgr := setupTestRouter()

	// 解析域名（会缓存）
	mgr.ResolveDNS("cached.example.com", "A", "192.168.1.1", "")

	// 检查缓存大小
	assert.Greater(t, mgr.GetCacheSize(), 0)

	// 清除缓存
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/dns/cache", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证缓存已清除
	assert.Equal(t, 0, mgr.GetCacheSize())
}

// TestServiceStartStop 测试服务启停 API.
func TestServiceStartStop(t *testing.T) {
	r, _ := setupTestRouter()

	// 启动
	reqBody := map[string]interface{}{
		"listen_addr": "0.0.0.0",
		"udp_port":    5353,
		"tcp_port":    5353,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/dns/start", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 获取状态
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/dns/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, true, data["running"])

	// 停止
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/dns/stop", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
