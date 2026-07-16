package domainsync

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// APIHandlers 域同步 API 处理器.
type APIHandlers struct {
	manager *Manager
}

// NewAPIHandlers 创建 API 处理器.
func NewAPIHandlers(manager *Manager) *APIHandlers {
	return &APIHandlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *APIHandlers) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/domain-sync/ous", h.ListOUs)
	r.GET("/domain-sync/config", h.GetConfig)
	r.PUT("/domain-sync/config", h.UpdateConfig)
	r.POST("/domain-sync/sync", h.TriggerSync)
	r.GET("/domain-sync/status", h.GetStatus)
}

// ListOUs 列出所有 OU.
// GET /api/v1/domain-sync/ous.
func (h *APIHandlers) ListOUs(c *gin.Context) {
	ous, err := h.manager.ListOUs()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    ous,
		"total":   len(ous),
	})
}

// GetConfig 获取当前配置.
// GET /api/v1/domain-sync/config.
func (h *APIHandlers) GetConfig(c *gin.Context) {
	config := h.manager.GetConfig()

	// 隐藏敏感字段
	safeConfig := sanitizeConfig(config)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    safeConfig,
	})
}

// UpdateConfig 更新同步配置.
// PUT /api/v1/domain-sync/config.
func (h *APIHandlers) UpdateConfig(c *gin.Context) {
	var config SyncConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的请求体: " + err.Error(),
		})
		return
	}

	// 设置默认超时
	if config.DCConfig.ConnectTimeout == 0 {
		config.DCConfig.ConnectTimeout = 10 * time.Second
	}
	if config.PoolSize == 0 {
		config.PoolSize = 5
	}
	if config.ConflictResolution == "" {
		config.ConflictResolution = "merge"
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置已更新",
	})
}

// TriggerSync 手动触发同步.
// POST /api/v1/domain-sync/sync.
func (h *APIHandlers) TriggerSync(c *gin.Context) {
	result, err := h.manager.StartSync(c.Request.Context())
	if err != nil {
		if err == ErrSyncInProgress {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "同步正在进行中",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetStatus 获取同步状态.
// GET /api/v1/domain-sync/status.
func (h *APIHandlers) GetStatus(c *gin.Context) {
	status := h.manager.GetStatus()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// sanitizeConfig 清理敏感字段.
func sanitizeConfig(config SyncConfig) map[string]interface{} {
	dc := config.DCConfig
	// 隐藏密码
	maskedPassword := ""
	if dc.BindPassword != "" {
		maskedPassword = "******"
	}

	return map[string]interface{}{
		"dc_config": map[string]interface{}{
			"host":            dc.Host,
			"port":            dc.Port,
			"domain":          dc.Domain,
			"base_dn":         dc.BaseDN,
			"bind_dn":         dc.BindDN,
			"bind_password":   maskedPassword,
			"use_tls":         dc.UseTLS,
			"skip_tls_verify": dc.SkipTLSVerify,
			"connect_timeout": dc.ConnectTimeout.String(),
		},
		"strategy":            config.Strategy,
		"selected_ous":        config.SelectedOUs,
		"sync_users":          config.SyncUsers,
		"sync_groups":         config.SyncGroups,
		"schedule_interval":   config.ScheduleInterval.String(),
		"schedule_cron":       config.ScheduleCron,
		"conflict_resolution": config.ConflictResolution,
		"pool_size":           config.PoolSize,
	}
}
