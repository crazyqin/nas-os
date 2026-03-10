package network

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 网络 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建网络处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	network := r.Group("/network")
	{
		// ========== 网络接口 ==========
		network.GET("/interfaces", h.listInterfaces)
		network.GET("/interfaces/:name", h.getInterface)
		network.PUT("/interfaces/:name", h.configureInterface)
		network.POST("/interfaces/:name/toggle", h.toggleInterface)
		network.GET("/stats", h.getNetworkStats)

		// ========== DDNS ==========
		network.GET("/ddns", h.listDDNS)
		network.POST("/ddns", h.addDDNS)
		network.GET("/ddns/:domain", h.getDDNS)
		network.PUT("/ddns/:domain", h.updateDDNS)
		network.DELETE("/ddns/:domain", h.deleteDDNS)
		network.POST("/ddns/:domain/enable", h.enableDDNS)
		network.POST("/ddns/:domain/refresh", h.refreshDDNS)

		// ========== 端口转发 ==========
		network.GET("/portforwards", h.listPortForwards)
		network.POST("/portforwards", h.addPortForward)
		network.GET("/portforwards/:name", h.getPortForward)
		network.PUT("/portforwards/:name", h.updatePortForward)
		network.DELETE("/portforwards/:name", h.deletePortForward)
		network.POST("/portforwards/:name/enable", h.enablePortForward)
		network.GET("/portforwards/:name/status", h.getPortForwardStatus)
		network.GET("/portforwards/active", h.listActivePortForwards)

		// ========== 防火墙 ==========
		network.GET("/firewall/rules", h.listFirewallRules)
		network.POST("/firewall/rules", h.addFirewallRule)
		network.GET("/firewall/rules/:name", h.getFirewallRule)
		network.PUT("/firewall/rules/:name", h.updateFirewallRule)
		network.DELETE("/firewall/rules/:name", h.deleteFirewallRule)
		network.POST("/firewall/rules/:name/enable", h.enableFirewallRule)
		network.GET("/firewall/status", h.getFirewallStatus)
		network.GET("/firewall/active", h.listActiveFirewallRules)
		network.POST("/firewall/policy", h.setDefaultPolicy)
		network.POST("/firewall/flush", h.flushRules)
		network.POST("/firewall/save", h.saveFirewallRules)
		network.POST("/firewall/restore", h.restoreFirewallRules)
	}
}

// ========== 网络接口 API ==========

func (h *Handlers) listInterfaces(c *gin.Context) {
	ifaces, err := h.manager.ListInterfaces()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    ifaces,
	})
}

func (h *Handlers) getInterface(c *gin.Context) {
	name := c.Param("name")
	iface, err := h.manager.GetInterface(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    iface,
	})
}

func (h *Handlers) configureInterface(c *gin.Context) {
	name := c.Param("name")
	var config InterfaceConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.ConfigureInterface(name, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "配置成功"})
}

func (h *Handlers) toggleInterface(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Up bool `json:"up"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.ToggleInterface(name, req.Up); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	action := "已禁用"
	if req.Up {
		action = "已启用"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "接口 " + action})
}

func (h *Handlers) getNetworkStats(c *gin.Context) {
	stats, err := h.manager.GetNetworkStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ========== DDNS API ==========

func (h *Handlers) listDDNS(c *gin.Context) {
	configs := h.manager.ListDDNS()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    configs,
	})
}

func (h *Handlers) addDDNS(c *gin.Context) {
	var config DDNSConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.AddDDNS(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "DDNS 配置已添加"})
}

func (h *Handlers) getDDNS(c *gin.Context) {
	domain := c.Param("domain")
	config, err := h.manager.GetDDNS(domain)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

func (h *Handlers) updateDDNS(c *gin.Context) {
	domain := c.Param("domain")
	var config DDNSConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.UpdateDDNS(domain, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "DDNS 配置已更新"})
}

func (h *Handlers) deleteDDNS(c *gin.Context) {
	domain := c.Param("domain")
	if err := h.manager.DeleteDDNS(domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "DDNS 配置已删除"})
}

func (h *Handlers) enableDDNS(c *gin.Context) {
	domain := c.Param("domain")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.EnableDDNS(domain, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	action := "已禁用"
	if req.Enabled {
		action = "已启用"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "DDNS " + action})
}

func (h *Handlers) refreshDDNS(c *gin.Context) {
	domain := c.Param("domain")
	if err := h.manager.RefreshDDNS(domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "DDNS 已刷新"})
}

// ========== 端口转发 API ==========

func (h *Handlers) listPortForwards(c *gin.Context) {
	rules := h.manager.ListPortForwards()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

func (h *Handlers) addPortForward(c *gin.Context) {
	var rule PortForward
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.AddPortForward(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "端口转发规则已添加"})
}

func (h *Handlers) getPortForward(c *gin.Context) {
	name := c.Param("name")
	rule, err := h.manager.GetPortForward(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rule,
	})
}

func (h *Handlers) updatePortForward(c *gin.Context) {
	name := c.Param("name")
	var rule PortForward
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.UpdatePortForward(name, rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "端口转发规则已更新"})
}

func (h *Handlers) deletePortForward(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeletePortForward(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "端口转发规则已删除"})
}

func (h *Handlers) enablePortForward(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.EnablePortForward(name, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	action := "已禁用"
	if req.Enabled {
		action = "已启用"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "端口转发规则 " + action})
}

func (h *Handlers) getPortForwardStatus(c *gin.Context) {
	name := c.Param("name")
	status, err := h.manager.GetPortForwardStatus(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"status": status},
	})
}

func (h *Handlers) listActivePortForwards(c *gin.Context) {
	rules, err := h.manager.ListActivePortForwards()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// ========== 防火墙 API ==========

func (h *Handlers) listFirewallRules(c *gin.Context) {
	rules := h.manager.ListFirewallRules()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

func (h *Handlers) addFirewallRule(c *gin.Context) {
	var rule FirewallRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.AddFirewallRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "防火墙规则已添加"})
}

func (h *Handlers) getFirewallRule(c *gin.Context) {
	name := c.Param("name")
	rule, err := h.manager.GetFirewallRule(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rule,
	})
}

func (h *Handlers) updateFirewallRule(c *gin.Context) {
	name := c.Param("name")
	var rule FirewallRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.UpdateFirewallRule(name, rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "防火墙规则已更新"})
}

func (h *Handlers) deleteFirewallRule(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeleteFirewallRule(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "防火墙规则已删除"})
}

func (h *Handlers) enableFirewallRule(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.EnableFirewallRule(name, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	action := "已禁用"
	if req.Enabled {
		action = "已启用"
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "防火墙规则 " + action})
}

func (h *Handlers) getFirewallStatus(c *gin.Context) {
	status, err := h.manager.GetFirewallStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

func (h *Handlers) listActiveFirewallRules(c *gin.Context) {
	rules, err := h.manager.ListActiveFirewallRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

func (h *Handlers) setDefaultPolicy(c *gin.Context) {
	var req struct {
		Chain  string `json:"chain" binding:"required"`
		Policy string `json:"policy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.SetDefaultPolicy(req.Chain, req.Policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "默认策略已设置"})
}

func (h *Handlers) flushRules(c *gin.Context) {
	var req struct {
		Chain string `json:"chain"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.FlushRules(req.Chain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "规则已清空"})
}

func (h *Handlers) saveFirewallRules(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	c.ShouldBindJSON(&req)

	if err := h.manager.SaveFirewallRules(req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "防火墙规则已保存"})
}

func (h *Handlers) restoreFirewallRules(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	c.ShouldBindJSON(&req)

	if err := h.manager.RestoreFirewallRules(req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "防火墙规则已恢复"})
}