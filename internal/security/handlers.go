package security

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 安全模块 HTTP 处理器
type Handlers struct {
	manager *SecurityManager
}

// NewHandlers 创建安全处理器
func NewHandlers(mgr *SecurityManager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
// 注意：调用方应在应用此路由组前添加认证和权限中间件
// 这些都是敏感的安全操作，应该限制为管理员权限
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	security := api.Group("/security")
	// 安全操作需要管理员权限，调用方应添加相应中间件
	{
		// 仪表板
		security.GET("/dashboard", h.getDashboard)

		// 配置 - 需要管理员权限
		security.GET("/config", h.getConfig)
		security.PUT("/config", h.updateConfig) // 需要管理员权限

		// ========== 防火墙 ========== // 需要管理员权限
		firewall := security.Group("/firewall")
		{
			firewall.GET("/status", h.getFirewallStatus)
			firewall.GET("/rules", h.listFirewallRules)
			firewall.POST("/rules", h.addFirewallRule)          // 需要管理员权限
			firewall.PUT("/rules/:id", h.updateFirewallRule)    // 需要管理员权限
			firewall.DELETE("/rules/:id", h.deleteFirewallRule) // 需要管理员权限

			firewall.GET("/blacklist", h.getBlacklist)
			firewall.POST("/blacklist", h.addToBlacklist)            // 需要管理员权限
			firewall.DELETE("/blacklist/:ip", h.removeFromBlacklist) // 需要管理员权限

			firewall.GET("/whitelist", h.getWhitelist)
			firewall.POST("/whitelist", h.addToWhitelist)            // 需要管理员权限
			firewall.DELETE("/whitelist/:ip", h.removeFromWhitelist) // 需要管理员权限
		}

		// ========== 失败登录保护 ========== // 需要管理员权限
		fail2ban := security.Group("/fail2ban")
		{
			fail2ban.GET("/status", h.getFail2BanStatus)
			fail2ban.PUT("/config", h.updateFail2BanConfig) // 需要管理员权限
			fail2ban.GET("/banned", h.getBannedIPs)
			fail2ban.POST("/unban/:ip", h.unbanIP) // 需要管理员权限
			fail2ban.GET("/attempts/:ip", h.getFailedAttempts)
		}

		// ========== 安全审计 ==========
		audit := security.Group("/audit")
		{
			audit.GET("/logs", h.getAuditLogs)
			audit.GET("/login-logs", h.getLoginLogs)
			audit.GET("/alerts", h.getAlerts)
			audit.POST("/alerts/:id/acknowledge", h.acknowledgeAlert) // 需要管理员权限
			audit.GET("/stats", h.getAuditStats)
			audit.GET("/export", h.exportLogs)
		}

		// ========== 安全基线 ==========
		baseline := security.Group("/baseline")
		{
			baseline.GET("/checks", h.getBaselineChecks)
			baseline.GET("/check", h.runBaselineCheck)
			baseline.GET("/report", h.getBaselineReport)
			baseline.GET("/categories", h.getBaselineCategories)
		}
	}
}

// ========== 通用响应 ==========

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// success 创建成功响应
func success(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

// apiError 创建错误响应
func apiError(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// ========== 仪表板 ==========

func (h *Handlers) getDashboard(c *gin.Context) {
	dashboard := h.manager.GetDashboard()
	c.JSON(http.StatusOK, success(dashboard))
}

// ========== 配置 ==========

func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, success(config))
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var config SecurityConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// ========== 防火墙 ==========

func (h *Handlers) getFirewallStatus(c *gin.Context) {
	status := h.manager.GetFirewallManager().GetConfig()
	c.JSON(http.StatusOK, success(status))
}

func (h *Handlers) listFirewallRules(c *gin.Context) {
	rules := h.manager.GetFirewallManager().ListRules()
	c.JSON(http.StatusOK, success(rules))
}

type AddFirewallRuleRequest struct {
	Name        string `json:"name" binding:"required"`
	Enabled     bool   `json:"enabled"`
	Action      string `json:"action" binding:"required"` // allow, deny, drop
	Protocol    string `json:"protocol"`                  // tcp, udp, icmp, all
	SourceIP    string `json:"source_ip"`
	DestIP      string `json:"dest_ip"`
	SourcePort  string `json:"source_port"`
	DestPort    string `json:"dest_port"`
	Direction   string `json:"direction"` // inbound, outbound
	Interface   string `json:"interface"`
	GeoLocation string `json:"geo_location"`
	Priority    int    `json:"priority"`
}

func (h *Handlers) addFirewallRule(c *gin.Context) {
	var req AddFirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	rule := FirewallRule{
		Name:        req.Name,
		Enabled:     req.Enabled,
		Action:      req.Action,
		Protocol:    req.Protocol,
		SourceIP:    req.SourceIP,
		DestIP:      req.DestIP,
		SourcePort:  req.SourcePort,
		DestPort:    req.DestPort,
		Direction:   req.Direction,
		Interface:   req.Interface,
		GeoLocation: req.GeoLocation,
		Priority:    req.Priority,
	}

	created, err := h.manager.GetFirewallManager().AddRule(rule)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 记录审计日志
	userID := c.GetString("user_id")
	username := c.GetString("username")
	ip := c.ClientIP()
	h.manager.RecordAction(userID, username, ip, "firewall_rule", "create", map[string]interface{}{
		"rule_id":   created.ID,
		"rule_name": created.Name,
	}, "success")

	c.JSON(http.StatusCreated, success(created))
}

func (h *Handlers) updateFirewallRule(c *gin.Context) {
	ruleID := c.Param("id")

	var req AddFirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	rule := FirewallRule{
		Name:        req.Name,
		Enabled:     req.Enabled,
		Action:      req.Action,
		Protocol:    req.Protocol,
		SourceIP:    req.SourceIP,
		DestIP:      req.DestIP,
		SourcePort:  req.SourcePort,
		DestPort:    req.DestPort,
		Direction:   req.Direction,
		Interface:   req.Interface,
		GeoLocation: req.GeoLocation,
		Priority:    req.Priority,
	}

	updated, err := h.manager.GetFirewallManager().UpdateRule(ruleID, rule)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 记录审计日志
	userID := c.GetString("user_id")
	username := c.GetString("username")
	ip := c.ClientIP()
	h.manager.RecordAction(userID, username, ip, "firewall_rule", "update", map[string]interface{}{
		"rule_id":   ruleID,
		"rule_name": updated.Name,
	}, "success")

	c.JSON(http.StatusOK, success(updated))
}

func (h *Handlers) deleteFirewallRule(c *gin.Context) {
	ruleID := c.Param("id")

	if err := h.manager.GetFirewallManager().DeleteRule(ruleID); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 记录审计日志
	userID := c.GetString("user_id")
	username := c.GetString("username")
	ip := c.ClientIP()
	h.manager.RecordAction(userID, username, ip, "firewall_rule", "delete", map[string]interface{}{
		"rule_id": ruleID,
	}, "success")

	c.JSON(http.StatusOK, success(nil))
}

// ========== IP 黑名单 ==========

func (h *Handlers) getBlacklist(c *gin.Context) {
	blacklist := h.manager.GetFirewallManager().GetBlacklist()
	c.JSON(http.StatusOK, success(blacklist))
}

type AddToBlacklistRequest struct {
	IP       string `json:"ip" binding:"required"`
	Reason   string `json:"reason"`
	Duration int    `json:"duration_minutes"` // 0 表示永久
}

func (h *Handlers) addToBlacklist(c *gin.Context) {
	var req AddToBlacklistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetFirewallManager().AddToBlacklist(req.IP, req.Reason, req.Duration); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 记录审计日志
	userID := c.GetString("user_id")
	username := c.GetString("username")
	ip := c.ClientIP()
	h.manager.RecordAction(userID, username, ip, "ip_blacklist", "add", map[string]interface{}{
		"target_ip": req.IP,
		"reason":    req.Reason,
	}, "success")

	c.JSON(http.StatusOK, success(nil))
}

func (h *Handlers) removeFromBlacklist(c *gin.Context) {
	ip := c.Param("ip")

	if err := h.manager.GetFirewallManager().RemoveFromBlacklist(ip); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 记录审计日志
	userID := c.GetString("user_id")
	username := c.GetString("username")
	ip = c.ClientIP()
	h.manager.RecordAction(userID, username, ip, "ip_blacklist", "remove", map[string]interface{}{
		"target_ip": ip,
	}, "success")

	c.JSON(http.StatusOK, success(nil))
}

// ========== IP 白名单 ==========

func (h *Handlers) getWhitelist(c *gin.Context) {
	whitelist := h.manager.GetFirewallManager().GetWhitelist()
	c.JSON(http.StatusOK, success(whitelist))
}

type AddToWhitelistRequest struct {
	IP     string `json:"ip" binding:"required"`
	Reason string `json:"reason"`
}

func (h *Handlers) addToWhitelist(c *gin.Context) {
	var req AddToWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetFirewallManager().AddToWhitelist(req.IP, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *Handlers) removeFromWhitelist(c *gin.Context) {
	ip := c.Param("ip")

	if err := h.manager.GetFirewallManager().RemoveFromWhitelist(ip); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// ========== 失败登录保护 ==========

func (h *Handlers) getFail2BanStatus(c *gin.Context) {
	status := h.manager.GetFail2BanManager().GetStatus()
	c.JSON(http.StatusOK, success(status))
}

func (h *Handlers) updateFail2BanConfig(c *gin.Context) {
	var config Fail2BanConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	if err := h.manager.GetFail2BanManager().UpdateConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *Handlers) getBannedIPs(c *gin.Context) {
	banned := h.manager.GetFail2BanManager().GetBannedIPs()
	c.JSON(http.StatusOK, success(banned))
}

func (h *Handlers) unbanIP(c *gin.Context) {
	ip := c.Param("ip")

	if err := h.manager.UnbanIP(ip); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 记录审计日志
	userID := c.GetString("user_id")
	username := c.GetString("username")
	clientIP := c.ClientIP()
	h.manager.RecordAction(userID, username, clientIP, "fail2ban", "unban", map[string]interface{}{
		"target_ip": ip,
	}, "success")

	c.JSON(http.StatusOK, success(nil))
}

func (h *Handlers) getFailedAttempts(c *gin.Context) {
	ip := c.Param("ip")

	attempts := h.manager.GetFail2BanManager().GetFailedAttempts(ip)
	c.JSON(http.StatusOK, success(attempts))
}

// ========== 安全审计 ==========

func (h *Handlers) getAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filters := make(map[string]string)
	for _, key := range []string{"category", "level", "username", "status", "event", "ip"} {
		if value := c.Query(key); value != "" {
			filters[key] = value
		}
	}

	logs := h.manager.GetAuditManager().GetAuditLogs(limit, offset, filters)
	c.JSON(http.StatusOK, success(logs))
}

func (h *Handlers) getLoginLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filters := make(map[string]string)
	for _, key := range []string{"username", "status", "ip"} {
		if value := c.Query(key); value != "" {
			filters[key] = value
		}
	}

	logs := h.manager.GetAuditManager().GetLoginLogs(limit, offset, filters)
	c.JSON(http.StatusOK, success(logs))
}

func (h *Handlers) getAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var acknowledged *bool
	if ack := c.Query("acknowledged"); ack != "" {
		value := ack == "true"
		acknowledged = &value
	}

	alerts := h.manager.GetAuditManager().GetAlerts(limit, offset, acknowledged)
	c.JSON(http.StatusOK, success(alerts))
}

func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	alertID := c.Param("id")
	username := c.GetString("username")

	if err := h.manager.GetAuditManager().AcknowledgeAlert(alertID, username); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

func (h *Handlers) getAuditStats(c *gin.Context) {
	stats := h.manager.GetAuditManager().GetAlertStats()
	c.JSON(http.StatusOK, success(stats))
}

func (h *Handlers) exportLogs(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime time.Time
	var err error

	if startTimeStr != "" {
		startTime, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(400, "无效的开始时间"))
			return
		}
	} else {
		startTime = time.Now().Add(-24 * time.Hour)
	}

	if endTimeStr != "" {
		endTime, err = time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, apiError(400, "无效的结束时间"))
			return
		}
	} else {
		endTime = time.Now()
	}

	data, err := h.manager.GetAuditManager().ExportLogs(startTime, endTime, format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiError(500, err.Error()))
		return
	}

	contentType := "application/json"
	if format == "csv" {
		contentType = "text/csv"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=audit-logs."+format)
	c.Data(http.StatusOK, contentType, data)
}

// ========== 安全基线 ==========

func (h *Handlers) getBaselineChecks(c *gin.Context) {
	checks := h.manager.GetBaselineManager().GetCheckList()
	c.JSON(http.StatusOK, success(checks))
}

func (h *Handlers) runBaselineCheck(c *gin.Context) {
	category := c.Query("category")

	var report BaselineReport
	if category != "" {
		report = h.manager.GetBaselineManager().RunChecksByCategory(category)
	} else {
		report = h.manager.GetBaselineManager().RunAllChecks()
	}

	// 记录审计日志
	userID := c.GetString("user_id")
	username := c.GetString("username")
	ip := c.ClientIP()
	h.manager.RecordAction(userID, username, ip, "baseline_check", "run", map[string]interface{}{
		"category":      category,
		"total_checks":  report.TotalChecks,
		"passed":        report.Passed,
		"failed":        report.Failed,
		"overall_score": report.OverallScore,
	}, "success")

	c.JSON(http.StatusOK, success(report))
}

func (h *Handlers) getBaselineReport(c *gin.Context) {
	// 获取最新的基线报告
	report := h.manager.GetBaselineManager().RunAllChecks()
	c.JSON(http.StatusOK, success(report))
}

func (h *Handlers) getBaselineCategories(c *gin.Context) {
	categories := h.manager.GetBaselineManager().GetCategories()
	c.JSON(http.StatusOK, success(categories))
}
