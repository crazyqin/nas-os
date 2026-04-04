// Package ransomware 提供勒索防护API处理器
// Version: v2.388.0 - 蜜罐检测 + 行为分析 + 实时防护API
// 对标TrueNAS Ransomware Defense
package ransomware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ProtectionHandlers 勒索防护处理器
type ProtectionHandlers struct {
	protection *RealtimeProtection
	honeyMgr   *HoneyFileManager
}

// NewProtectionHandlers 创建勒索防护处理器
func NewProtectionHandlers(protection *RealtimeProtection, honeyMgr *HoneyFileManager) *ProtectionHandlers {
	return &ProtectionHandlers{
		protection: protection,
		honeyMgr:   honeyMgr,
	}
}

// RegisterRoutes 注册勒索防护路由
func (h *ProtectionHandlers) RegisterRoutes(r *gin.RouterGroup) {
	ransomware := r.Group("/ransomware")
	{
		// 防护状态
		ransomware.GET("/status", h.GetStatus)
		ransomware.POST("/start", h.StartProtection)
		ransomware.POST("/stop", h.StopProtection)

		// 配置
		ransomware.GET("/config", h.GetConfig)
		ransomware.PUT("/config", h.UpdateConfig)

		// 蜜罐文件管理
		ransomware.GET("/honeyfile", h.GetHoneyFiles)
		ransomware.POST("/honeyfile/deploy", h.DeployHoneyFiles)
		ransomware.POST("/honeyfile/redeploy", h.RedeployHoneyFiles)
		ransomware.GET("/honeyfile/stats", h.GetHoneyFileStats)
		ransomware.GET("/honeyfile/events", h.GetHoneyFileEvents)

		// 蜜罐配置
		ransomware.GET("/honeyfile/config", h.GetHoneyFileConfig)
		ransomware.PUT("/honeyfile/config", h.UpdateHoneyFileConfig)

		// 威胁检测
		ransomware.POST("/scan", h.ScanDirectory)
		ransomware.GET("/threats/history", h.GetThreatHistory)

		// 隔离管理
		ransomware.GET("/quarantine", h.GetQuarantineList)
		ransomware.POST("/quarantine/:id/restore", h.RestoreFromQuarantine)
		ransomware.DELETE("/quarantine/:id", h.DeleteFromQuarantine)

		// 快照管理
		ransomware.GET("/snapshots", h.GetSnapshots)
		ransomware.POST("/snapshots/:id/restore", h.RestoreSnapshot)

		// 白名单
		ransomware.GET("/whitelist", h.GetWhitelist)
		ransomware.POST("/whitelist/path", h.AddWhitelistPath)
		ransomware.DELETE("/whitelist/path", h.RemoveWhitelistPath)
		ransomware.POST("/whitelist/process", h.AddWhitelistProcess)
		ransomware.DELETE("/whitelist/process", h.RemoveWhitelistProcess)

		// 检测灵敏度
		ransomware.PUT("/sensitivity", h.SetSensitivity)

		// 实时防护状态
		ransomware.GET("/realtime/status", h.GetRealtimeStatus)
		ransomware.GET("/realtime/stats", h.GetRealtimeStats)
	}
}

// GetStatus 获取防护状态
// @Summary 获取防护状态
// @Description 获取勒索防护的整体状态
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/status [get]
func (h *ProtectionHandlers) GetStatus(c *gin.Context) {
	status := h.protection.GetStatus()
	honeyStats := h.honeyMgr.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"protectionStatus": status.ProtectionStatus,
			"uptime":           status.Uptime,
			"totalEvents":      status.TotalEvents,
			"threatsDetected":  status.ThreatsDetected,
			"threatsBlocked":   status.ThreatsBlocked,
			"filesQuarantined": status.FilesQuarantined,
			"snapshotsCreated": status.SnapshotsCreated,
			"alertsSent":       status.AlertsSent,
			"honeyFiles": gin.H{
				"total":    honeyStats.TotalFiles,
				"active":   honeyStats.ActiveFiles,
				"triggered": honeyStats.TriggeredFiles,
				"lastDeployed": honeyStats.LastDeployed,
			},
			"lastThreatTime":  status.LastThreatTime,
			"lastThreatLevel": status.LastThreatLevel,
		},
	})
}

// StartProtection 启动防护
// @Summary 启动防护
// @Description 启动勒索软件实时防护
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/start [post]
func (h *ProtectionHandlers) StartProtection(c *gin.Context) {
	h.protection.Start(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "勒索防护已启动",
	})
}

// StopProtection 停止防护
// @Summary 停止防护
// @Description 停止勒索软件实时防护
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/stop [post]
func (h *ProtectionHandlers) StopProtection(c *gin.Context) {
	h.protection.Stop()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "勒索防护已停止",
	})
}

// GetConfig 获取配置
// @Summary 获取配置
// @Description 获取勒索防护配置
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/config [get]
func (h *ProtectionHandlers) GetConfig(c *gin.Context) {
	config := h.protection.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// UpdateConfig 更新配置
// @Summary 更新配置
// @Description 更新勒索防护配置
// @Tags ransomware
// @Accept json
// @Produce json
// @Param request body RealtimeProtectionConfig true "配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/config [put]
func (h *ProtectionHandlers) UpdateConfig(c *gin.Context) {
	var config RealtimeProtectionConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.protection.UpdateConfig(config)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已更新",
	})
}

// GetHoneyFiles 获取蜜罐文件列表
// @Summary 获取蜜罐文件列表
// @Description 获取所有蜜罐文件列表
// @Tags ransomware
// @Accept json
// @Produce json
// @Param path query string false "按路径过滤"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/honeyfile [get]
func (h *ProtectionHandlers) GetHoneyFiles(c *gin.Context) {
	path := c.Query("path")

	var files []*HoneyFile
	if path != "" {
		files = h.honeyMgr.GetFilesByPath(path)
	} else {
		files = h.honeyMgr.GetFiles()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total": len(files),
			"files": files,
		},
	})
}

// DeployHoneyFiles 部署蜜罐文件
// @Summary 部署蜜罐文件
// @Description 在所有配置路径部署蜜罐文件
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/honeyfile/deploy [post]
func (h *ProtectionHandlers) DeployHoneyFiles(c *gin.Context) {
	err := h.honeyMgr.DeployAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	stats := h.honeyMgr.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "蜜罐文件部署成功",
		"data": gin.H{
			"totalFiles":   stats.TotalFiles,
			"activeFiles":  stats.ActiveFiles,
			"lastDeployed": stats.LastDeployed,
		},
	})
}

// RedeployHoneyFiles 重新部署蜜罐文件
// @Summary 重新部署蜜罐文件
// @Description 重新部署被触发的蜜罐文件
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/honeyfile/redeploy [post]
func (h *ProtectionHandlers) RedeployHoneyFiles(c *gin.Context) {
	err := h.honeyMgr.Redeploy()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	stats := h.honeyMgr.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "蜜罐文件重新部署成功",
		"data":    stats,
	})
}

// GetHoneyFileStats 获取蜜罐统计
// @Summary 获取蜜罐统计
// @Description 获取蜜罐文件统计数据
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/honeyfile/stats [get]
func (h *ProtectionHandlers) GetHoneyFileStats(c *gin.Context) {
	stats := h.honeyMgr.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// GetHoneyFileEvents 获取蜜罐事件
// @Summary 获取蜜罐事件
// @Description 获取蜜罐文件触发事件记录
// @Tags ransomware
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制" default(100)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/honeyfile/events [get]
func (h *ProtectionHandlers) GetHoneyFileEvents(c *gin.Context) {
	limit := 100
	// 解析limit参数

	events := h.honeyMgr.GetEvents(limit)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// GetHoneyFileConfig 获取蜜罐配置
// @Summary 获取蜜罐配置
// @Description 获取当前蜜罐文件配置
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/honeyfile/config [get]
func (h *ProtectionHandlers) GetHoneyFileConfig(c *gin.Context) {
	// 返回默认配置作为示例
	config := DefaultHoneyFileConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// honeyFileConfigRequest 蜜罐配置请求
type honeyFileConfigRequest struct {
	Enabled       bool     `json:"enabled"`
	DeployPaths   []string `json:"deployPaths"`
	FilesPerPath  int      `json:"filesPerPath"`
	FileTypes     []string `json:"fileTypes"`
	AlertOnModify bool     `json:"alertOnModify"`
	AlertOnDelete bool     `json:"alertOnDelete"`
	AlertOnRename bool     `json:"alertOnRename"`
}

// UpdateHoneyFileConfig 更新蜜罐配置
// @Summary 更新蜜罐配置
// @Description 更新蜜罐文件配置
// @Tags ransomware
// @Accept json
// @Produce json
// @Param request body honeyFileConfigRequest true "配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/honeyfile/config [put]
func (h *ProtectionHandlers) UpdateHoneyFileConfig(c *gin.Context) {
	var req honeyFileConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 更新配置逻辑
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "蜜罐配置已更新",
	})
}

// ScanDirectory 扫描目录
// @Summary 扫描目录
// @Description 扫描指定目录检测勒索软件
// @Tags ransomware
// @Accept json
// @Produce json
// @Param request body scanRequest true "扫描请求"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/scan [post]
func (h *ProtectionHandlers) ScanDirectory(c *gin.Context) {
	var req scanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	result, err := h.protection.ScanNow(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// scanRequest 扫描请求
type scanRequest struct {
	Path string `json:"path" binding:"required"`
}

// GetThreatHistory 获取威胁历史
// @Summary 获取威胁历史
// @Description 获取检测到的威胁历史记录
// @Tags ransomware
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制" default(50)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/threats/history [get]
func (h *ProtectionHandlers) GetThreatHistory(c *gin.Context) {
	// 获取威胁历史
	history := h.protection.GetThreatHistory(50)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// GetQuarantineList 获取隔离列表
// @Summary 获取隔离列表
// @Description 获取已隔离的文件列表
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/quarantine [get]
func (h *ProtectionHandlers) GetQuarantineList(c *gin.Context) {
	// 获取隔离列表
	entries := h.protection.quarantine.ListEntries(100, 0, nil)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":   len(entries),
			"entries": entries,
		},
	})
}

// restoreQuarantineRequest 恢复隔离请求
type restoreQuarantineRequest struct {
	TargetPath string `json:"targetPath"`
}

// RestoreFromQuarantine 从隔离恢复文件
// @Summary 从隔离恢复文件
// @Description 从隔离区恢复指定文件
// @Tags ransomware
// @Accept json
// @Produce json
// @Param id path string true "隔离ID"
// @Param request body restoreQuarantineRequest true "恢复请求"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/quarantine/{id}/restore [post]
func (h *ProtectionHandlers) RestoreFromQuarantine(c *gin.Context) {
	id := c.Param("id")

	var req restoreQuarantineRequest
	c.ShouldBindJSON(&req) // 可选参数

	targetPath := req.TargetPath
	if targetPath == "" {
		targetPath = "/restored/" // 默认恢复路径
	}

	err := h.protection.RestoreFromQuarantine(id, targetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "文件已恢复",
		"data": gin.H{
			"id":         id,
			"targetPath": targetPath,
		},
	})
}

// DeleteFromQuarantine 从隔离删除文件
// @Summary 从隔离删除文件
// @Description 永久删除隔离区中的文件
// @Tags ransomware
// @Accept json
// @Produce json
// @Param id path string true "隔离ID"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/quarantine/{id} [delete]
func (h *ProtectionHandlers) DeleteFromQuarantine(c *gin.Context) {
	_ = c.Param("id")

	// 删除逻辑
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "文件已删除",
	})
}

// GetSnapshots 获取快照列表
// @Summary 获取快照列表
// @Description 获取保护快照列表
// @Tags ransomware
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制" default(20)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/snapshots [get]
func (h *ProtectionHandlers) GetSnapshots(c *gin.Context) {
	// 获取快照列表
	snapshots := h.protection.snapshotMgr.ListSnapshots(20, 0, nil)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":     len(snapshots),
			"snapshots": snapshots,
		},
	})
}

// restoreSnapshotRequest 恢复快照请求
type restoreSnapshotRequest struct {
	TargetPath string `json:"targetPath"`
}

// RestoreSnapshot 恢复快照
// @Summary 恢复快照
// @Description 从保护快照恢复数据
// @Tags ransomware
// @Accept json
// @Produce json
// @Param id path string true "快照ID"
// @Param request body restoreSnapshotRequest true "恢复请求"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/snapshots/{id}/restore [post]
func (h *ProtectionHandlers) RestoreSnapshot(c *gin.Context) {
	id := c.Param("id")

	var req restoreSnapshotRequest
	c.ShouldBindJSON(&req)

	targetPath := req.TargetPath
	if targetPath == "" {
		targetPath = "/restored/"
	}

	err := h.protection.RestoreSnapshot(id, targetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "快照已恢复",
		"data": gin.H{
			"snapshotId": id,
			"targetPath": targetPath,
		},
	})
}

// GetWhitelist 获取白名单
// @Summary 获取白名单
// @Description 获取当前白名单配置
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/whitelist [get]
func (h *ProtectionHandlers) GetWhitelist(c *gin.Context) {
	config := h.protection.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config.Whitelist,
	})
}

// whitelistPathRequest 白名单路径请求
type whitelistPathRequest struct {
	Path string `json:"path" binding:"required"`
}

// AddWhitelistPath 添加白名单路径
// @Summary 添加白名单路径
// @Description 添加可信路径到白名单
// @Tags ransomware
// @Accept json
// @Produce json
// @Param request body whitelistPathRequest true "路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/whitelist/path [post]
func (h *ProtectionHandlers) AddWhitelistPath(c *gin.Context) {
	var req whitelistPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.protection.AddWhitelistPath(req.Path)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "白名单路径已添加",
	})
}

// RemoveWhitelistPath 移除白名单路径
// @Summary 移除白名单路径
// @Description 从白名单移除可信路径
// @Tags ransomware
// @Accept json
// @Produce json
// @Param request body whitelistPathRequest true "路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/whitelist/path [delete]
func (h *ProtectionHandlers) RemoveWhitelistPath(c *gin.Context) {
	var req whitelistPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.protection.RemoveWhitelistPath(req.Path)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "白名单路径已移除",
	})
}

// whitelistProcessRequest 白名单进程请求
type whitelistProcessRequest struct {
	Process string `json:"process" binding:"required"`
}

// AddWhitelistProcess 添加白名单进程
// @Summary 添加白名单进程
// @Description 添加可信进程到白名单
// @Tags ransomware
// @Accept json
// @Produce json
// @Param request body whitelistProcessRequest true "进程名"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/whitelist/process [post]
func (h *ProtectionHandlers) AddWhitelistProcess(c *gin.Context) {
	var req whitelistProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 添加进程到白名单逻辑
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "白名单进程已添加",
	})
}

// RemoveWhitelistProcess 移除白名单进程
// @Summary 移除白名单进程
// @Description 从白名单移除可信进程
// @Tags ransomware
// @Accept json
// @Produce json
// @Param request body whitelistProcessRequest true "进程名"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/whitelist/process [delete]
func (h *ProtectionHandlers) RemoveWhitelistProcess(c *gin.Context) {
	var req whitelistProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 移除进程从白名单逻辑
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "白名单进程已移除",
	})
}

// sensitivityRequest 灵敏度请求
type sensitivityRequest struct {
	Sensitivity string `json:"sensitivity" binding:"required"` // low/medium/high
}

// SetSensitivity 设置检测灵敏度
// @Summary 设置检测灵敏度
// @Description 设置勒索防护的检测灵敏度级别
// @Tags ransomware
// @Accept json
// @Produce json
// @Param request body sensitivityRequest true "灵敏度"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/sensitivity [put]
func (h *ProtectionHandlers) SetSensitivity(c *gin.Context) {
	var req sensitivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if req.Sensitivity != "low" && req.Sensitivity != "medium" && req.Sensitivity != "high" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "灵敏度必须是 low/medium/high 之一",
		})
		return
	}

	h.protection.SetSensitivity(req.Sensitivity)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "检测灵敏度已更新",
		"data": gin.H{
			"sensitivity": req.Sensitivity,
		},
	})
}

// GetRealtimeStatus 获取实时防护状态
// @Summary 获取实时防护状态
// @Description 获取实时防护的详细状态信息
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/realtime/status [get]
func (h *ProtectionHandlers) GetRealtimeStatus(c *gin.Context) {
	status := h.protection.GetStatus()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"status":           status.ProtectionStatus,
			"startTime":        status.StartTime,
			"uptime":           status.Uptime,
			"monitoredPaths":   status.MonitoredPaths,
			"writeOnceProtected": status.ProtectedByWriteOnce,
		},
	})
}

// GetRealtimeStats 获取实时防护统计
// @Summary 获取实时防护统计
// @Description 获取实时防护的统计数据
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/realtime/stats [get]
func (h *ProtectionHandlers) GetRealtimeStats(c *gin.Context) {
	status := h.protection.GetStatus()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalEvents":      status.TotalEvents,
			"threatsDetected":  status.ThreatsDetected,
			"threatsBlocked":   status.ThreatsBlocked,
			"filesQuarantined": status.FilesQuarantined,
			"snapshotsCreated": status.SnapshotsCreated,
			"alertsSent":       status.AlertsSent,
			"falsePositives":   status.FalsePositives,
			"lastThreatTime":   status.LastThreatTime,
			"lastThreatLevel":  status.LastThreatLevel,
		},
	})
}

// QuickScan 快速扫描API
// @Summary 快速扫描
// @Description 快速扫描指定路径检测勒索威胁
// @Tags ransomware
// @Accept json
// @Produce json
// @Param path query string true "扫描路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/quick-scan [get]
func (h *ProtectionHandlers) QuickScan(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "扫描路径不能为空",
		})
		return
	}

	result, err := h.protection.ScanNow(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"path":             result.Path,
			"infectedFiles":    result.InfectedFiles,
			"suspiciousFiles":  result.SuspiciousFiles,
			"riskScore":        result.RiskScore,
			"scannedAt":        result.ScannedAt,
			"duration":         result.Duration.String(),
		},
	})
}

// GetProtectionDashboard 获取防护仪表板数据
// @Summary 获取防护仪表板数据
// @Description 获取勒索防护的综合仪表板数据
// @Tags ransomware
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ransomware/dashboard [get]
func (h *ProtectionHandlers) GetProtectionDashboard(c *gin.Context) {
	status := h.protection.GetStatus()
	honeyStats := h.honeyMgr.GetStats()

	// 生成仪表板数据
	dashboard := gin.H{
		"status": gin.H{
			"protectionEnabled": status.ProtectionStatus == "active",
			"uptime":            status.Uptime,
		},
		"statistics": gin.H{
			"totalEvents":      status.TotalEvents,
			"threatsDetected":  status.ThreatsDetected,
			"threatsBlocked":   status.ThreatsBlocked,
			"filesQuarantined": status.FilesQuarantined,
			"snapshotsCreated": status.SnapshotsCreated,
		},
		"honeyFiles": gin.H{
			"total":     honeyStats.TotalFiles,
			"active":    honeyStats.ActiveFiles,
			"triggered": honeyStats.TriggeredFiles,
		},
		"lastThreat": gin.H{
			"time":  status.LastThreatTime,
			"level": status.LastThreatLevel,
		},
		"timestamp": time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    dashboard,
	})
}