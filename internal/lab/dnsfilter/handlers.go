// Package dnsfilter 提供 REST API 处理器
package dnsfilter

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers DNS 过滤模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/dns 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	dns := r.Group("/dns")
	{
		// 服务状态与控制
		dns.GET("/status", h.getStatus)
		dns.POST("/start", h.startService)
		dns.POST("/stop", h.stopService)

		// DNS 记录 CRUD
		dns.GET("/records", h.listDNSRecords)
		dns.POST("/records", h.createDNSRecord)
		dns.GET("/records/:id", h.getDNSRecord)
		dns.PUT("/records/:id", h.updateDNSRecord)
		dns.DELETE("/records/:id", h.deleteDNSRecord)

		// 过滤规则列表 CRUD
		dns.GET("/lists", h.listFilterLists)
		dns.POST("/lists", h.createFilterList)
		dns.GET("/lists/:id", h.getFilterList)
		dns.PUT("/lists/:id", h.updateFilterList)
		dns.DELETE("/lists/:id", h.deleteFilterList)
		dns.POST("/lists/:id/subscribe", h.subscribeFilterList)

		// 过滤规则 CRUD
		dns.GET("/rules", h.listFilterRules)
		dns.POST("/rules", h.createFilterRule)
		dns.DELETE("/rules/:id", h.deleteFilterRule)

		// 上游 DNS 管理
		dns.GET("/upstreams", h.listUpstreamDNS)
		dns.POST("/upstreams", h.createUpstreamDNS)
		dns.PUT("/upstreams/:id", h.updateUpstreamDNS)
		dns.DELETE("/upstreams/:id", h.deleteUpstreamDNS)

		// 过滤策略
		dns.GET("/policies", h.listFilterPolicies)
		dns.POST("/policies", h.createFilterPolicy)
		dns.PUT("/policies/:id", h.updateFilterPolicy)
		dns.DELETE("/policies/:id", h.deleteFilterPolicy)

		// 查询日志
		dns.GET("/logs", h.getQueryLogs)
		dns.GET("/logs/stream", h.streamLogs)

		// 统计
		dns.GET("/stats", h.getStats)

		// 测试
		dns.POST("/test", h.testDNS)

		// 缓存管理
		dns.DELETE("/cache", h.clearCache)
	}
}

// ========== 服务状态与控制 ==========

func (h *Handlers) getStatus(c *gin.Context) {
	status := h.manager.GetStatus()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: status})
}

func (h *Handlers) startService(c *gin.Context) {
	var req struct {
		ListenAddr string `json:"listen_addr"`
		UDPPort    int    `json:"udp_port"`
		TCPPort    int    `json:"tcp_port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if req.ListenAddr == "" {
		req.ListenAddr = "0.0.0.0"
	}
	if req.UDPPort == 0 {
		req.UDPPort = 53
	}
	if req.TCPPort == 0 {
		req.TCPPort = 53
	}

	if err := h.manager.Start(req.ListenAddr, req.UDPPort, req.TCPPort); err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "started"})
}

func (h *Handlers) stopService(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		c.JSON(http.StatusConflict, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "stopped"})
}

// ========== DNS 记录处理 ==========

func (h *Handlers) listDNSRecords(c *gin.Context) {
	records := h.manager.ListDNSRecords()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(records),
			"records": records,
		},
	})
}

func (h *Handlers) createDNSRecord(c *gin.Context) {
	var req CreateDNSRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	record := h.manager.CreateDNSRecord(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: record})
}

func (h *Handlers) getDNSRecord(c *gin.Context) {
	id := c.Param("id")
	record, err := h.manager.GetDNSRecord(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: record})
}

func (h *Handlers) updateDNSRecord(c *gin.Context) {
	id := c.Param("id")
	var req UpdateDNSRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	record, err := h.manager.UpdateDNSRecord(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: record})
}

func (h *Handlers) deleteDNSRecord(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteDNSRecord(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 过滤规则列表处理 ==========

func (h *Handlers) listFilterLists(c *gin.Context) {
	lists := h.manager.ListFilterLists()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(lists),
			"lists": lists,
		},
	})
}

func (h *Handlers) createFilterList(c *gin.Context) {
	var req CreateFilterListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	fl := h.manager.CreateFilterList(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: fl})
}

func (h *Handlers) getFilterList(c *gin.Context) {
	id := c.Param("id")
	fl, err := h.manager.GetFilterList(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: fl})
}

func (h *Handlers) updateFilterList(c *gin.Context) {
	id := c.Param("id")
	var req UpdateFilterListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	fl, err := h.manager.UpdateFilterList(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: fl})
}

func (h *Handlers) deleteFilterList(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteFilterList(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) subscribeFilterList(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.SubscribeFilterList(id); err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "subscribed"})
}

// ========== 过滤规则处理 ==========

func (h *Handlers) listFilterRules(c *gin.Context) {
	listID := c.Query("list_id")
	rules := h.manager.ListFilterRules(listID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

func (h *Handlers) createFilterRule(c *gin.Context) {
	var req CreateFilterRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	rule := h.manager.CreateFilterRule(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: rule})
}

func (h *Handlers) deleteFilterRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteFilterRule(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 上游 DNS 处理 ==========

func (h *Handlers) listUpstreamDNS(c *gin.Context) {
	upstreams := h.manager.ListUpstreamDNS()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(upstreams),
			"upstreams": upstreams,
		},
	})
}

func (h *Handlers) createUpstreamDNS(c *gin.Context) {
	var req CreateUpstreamDNSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	upstream := h.manager.CreateUpstreamDNS(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: upstream})
}

func (h *Handlers) updateUpstreamDNS(c *gin.Context) {
	id := c.Param("id")
	var req UpdateUpstreamDNSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	upstream, err := h.manager.UpdateUpstreamDNS(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: upstream})
}

func (h *Handlers) deleteUpstreamDNS(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteUpstreamDNS(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 过滤策略处理 ==========

func (h *Handlers) listFilterPolicies(c *gin.Context) {
	policies := h.manager.ListFilterPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(policies),
			"policies": policies,
		},
	})
}

func (h *Handlers) createFilterPolicy(c *gin.Context) {
	var req CreateFilterPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	policy := h.manager.CreateFilterPolicy(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: policy})
}

func (h *Handlers) updateFilterPolicy(c *gin.Context) {
	id := c.Param("id")
	var req UpdateFilterPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	policy, err := h.manager.UpdateFilterPolicy(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: policy})
}

func (h *Handlers) deleteFilterPolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteFilterPolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 查询日志处理 ==========

func (h *Handlers) getQueryLogs(c *gin.Context) {
	var req QueryLogRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	logs := h.manager.GetQueryLogs(req)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(logs),
			"logs":  logs,
		},
	})
}

func (h *Handlers) streamLogs(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: "streaming not supported"})
		return
	}

	ch := h.manager.SubscribeLogStream()
	defer h.manager.UnsubscribeLogStream(ch)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			data := fmt.Sprintf("id: %s\nevent: query\ndata: {\"id\":\"%s\",\"timestamp\":\"%s\",\"client_ip\":\"%s\",\"domain\":\"%s\",\"type\":\"%s\",\"answer\":\"%s\",\"is_filtered\":%t,\"action\":\"%s\",\"duration\":%d}\n\n",
				event.ID, event.ID, event.Timestamp.Format("2006-01-02T15:04:05Z"), event.ClientIP,
				event.Domain, event.Type, event.Answer, event.IsFiltered, event.Action, event.Duration)
			c.Writer.Write([]byte(data))
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// ========== 统计处理 ==========

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== 测试处理 ==========

func (h *Handlers) testDNS(c *gin.Context) {
	var req TestDNSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	queryType := req.Type
	if queryType == "" {
		queryType = "A"
	}

	logEntry := h.manager.ResolveDNS(req.Domain, queryType, "127.0.0.1", "")

	resp := TestDNSResponse{
		Domain:     logEntry.Domain,
		Type:       logEntry.Type,
		IsFiltered: logEntry.IsFiltered,
		Action:     string(logEntry.Action),
		Source:     "upstream",
		RuleMatch:  logEntry.FilterRule,
		Duration:   logEntry.Duration,
	}

	if logEntry.Answer != "" {
		resp.Answers = []string{logEntry.Answer}
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: resp})
}

// ========== 缓存管理 ==========

func (h *Handlers) clearCache(c *gin.Context) {
	h.manager.ClearCache()
	c.JSON(http.StatusOK, response{Code: 0, Message: "cache cleared"})
}
